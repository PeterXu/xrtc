package webrtc

import (
	"bytes"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

type TcpServer struct {
	TAG   string
	owner ServiceOwner
	addr  string
	ln    net.Listener
	mtx   sync.Mutex
	pool  *util.GoPool
}

// tcp server (https/wss/webrtc-tcp)
func NewTcpServer(owner ServiceOwner, addr string) *TcpServer {
	const TAG = "[TCP]"
	log.Println(TAG, "listen tcp on: ", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(TAG, "listen tcp error=", err)
		return nil
	}

	if tcpL, ok := l.(*net.TCPListener); ok {
		log.Println(TAG, "set reuse addr")
		util.SetSocketReuseAddr(tcpL)
	}
	svr := &TcpServer{
		TAG:   TAG,
		owner: owner,
		addr:  addr,
		ln:    l,
		pool:  util.NewGoPool(1024),
	}
	go svr.Run()
	return svr
}

func (s *TcpServer) Close() {
}

func (s *TcpServer) Run() {
	defer s.ln.Close()

	// How long to sleep on accept failure??
	var tempDelay time.Duration

	for {
		// Wait for a connection.
		conn, err := s.ln.Accept()
		if err != nil {
			log.Warnln(s.TAG, "accept error=", err)
			break
		}

		if ne, ok := err.(net.Error); ok && ne.Temporary() {
			if tempDelay == 0 {
				tempDelay = 5 * time.Millisecond
			} else {
				tempDelay *= 2
			}
			if max := 1 * time.Second; tempDelay > max {
				tempDelay = max
			}
			log.Warn2f(s.TAG, "accept error: %v; retrying after %v", err, tempDelay)
			time.Sleep(tempDelay)
			continue
		}
		tempDelay = 0

		handler := NewTcpHandler(s.TAG, s.owner, conn)
		s.pool.Schedule(handler.Run)
	}
}

/// tcp handler

type TcpHandler struct {
	*OneServiceHandler
	TAG   string
	owner ServiceOwner
	conn  *util.NetConn
}

func NewTcpHandler(TAG string, owner ServiceOwner, conn net.Conn) *TcpHandler {
	return &TcpHandler{
		OneServiceHandler: NewOneServiceHandler(),
		TAG:               TAG,
		owner:             owner,
		conn:              util.NewNetConn(conn),
	}
}

func (h *TcpHandler) Run() {
	addr := h.conn.RemoteAddr()
	h.TAG += "[" + addr.String() + "]"
	if !h.Process() {
		// Close here only when failed
		h.conn.Close()
	}
}

func (h *TcpHandler) Process() bool {
	kClientHello := util.SslClientHello
	kClientLen := len(util.SslClientHello)
	kServerHello := util.SslServerHello

	data, err := h.conn.Peek(kClientLen)
	if len(data) < 3 || err != nil {
		log.Warn2f(h.TAG, "no enough data, err: %v", err)
		return false
	}

	prelen := 3
	prefix := data[0:prelen]

	//FIXME: how many bytes used here? (3bytes??)
	if bytes.Compare(prefix, kClientHello[0:prelen]) == 0 {
		if len(data) < kClientLen {
			log.Warn2f(h.TAG, "check ssl hello, len(%d) not enough < sslLen(%d)", len(data), kClientLen)
			return false
		}
	}

	if len(data) >= kClientLen && bytes.Compare(data[0:kClientLen], kClientHello) == 0 {
		log.Println(h.TAG, "setup ssltcp handshake for", h.conn.RemoteAddr())
		h.conn.Write(kServerHello)
		h.serveTCP()
	} else if bytes.Compare(prefix, kServerHello[0:prelen]) != 0 {
		log.Println(h.TAG, "setup http/rawtcp handshake for", h.conn.RemoteAddr())
		if util.CheckHttpRequest(prefix) {
			http.Serve(
				NewHttpListener(h.TAG, h.conn),
				NewHttpServeHandler(h.owner.Name(), h.owner.RestParams()),
			)
		} else {
			h.serveTCP()
		}
	} else {
		log.Println(h.TAG, "setup tls key/cert for", h.conn.RemoteAddr())
		cer, err := tls.LoadX509KeyPair(h.owner.GetSslFile())
		if err != nil {
			log.Warn2f(h.TAG, "load tls key/cert err: %v", err)
			return false
		}

		// fake tls.conn to plain conn here
		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		conn2 := tls.Server(h.conn, config)
		if err := conn2.Handshake(); err != nil {
			log.Warn2f(h.TAG, "check tls handshake err: %v", err)
			return false
		}

		conn3 := util.NewNetConn(conn2)
		if prefix, err = conn3.Peek(3); err != nil {
			log.Warn2f(h.TAG, "check tls read err: %v", err)
			return false
		}

		// now it is plain conn for tcp/http
		log.Println(h.TAG, "setup tls (http/tcp) for", h.conn.RemoteAddr(), string(prefix))
		h.conn = conn3
		if util.CheckHttpRequest(prefix) {
			http.Serve(
				NewHttpListener(h.TAG, h.conn),
				NewHttpServeHandler(h.owner.Name(), h.owner.RestParams()),
			)
		} else {
			h.serveTCP()
		}
	}

	log.Println(h.TAG, "Process success")
	return true
}

func (h *TcpHandler) serveTCP() {
	log.Println(h.TAG, "main begin")

	defer h.conn.Close()

	go h.writeLoop()

	rbuf := make([]byte, 1024*128)
	for {
		if nret, err := util.ReadIceTcpPacket(h.conn, rbuf[0:]); err == nil {
			if nret > 0 {
				h.netStat.UpdateRecv(nret)
				h.owner.OnRecvData(rbuf[0:nret], h.conn.RemoteAddr(), h)
			} else {
				log.Warnln(h.TAG, "read data nothing")
			}
		} else {
			log.Warnln(h.TAG, "read data fail:", err)
			break
		}
	}

	h.exitTick <- true

	// TODO: remove connection/user/service from maxhub

	log.Println(h.TAG, "main end")
}

func (h *TcpHandler) writeLoop() {
	tickChan := time.NewTicker(time.Second * 10).C

exitLoop:
	for {
		select {
		case msg, ok := <-h.chanFeed:
			if !ok {
				log.Println(h.TAG, "close channel")
				break exitLoop
			}

			if umsg, ok := msg.(*ObjMessage); ok {
				if err, nb := h.sendInternal(umsg.data); err != nil {
					log.Warnln(h.TAG, "send err:", err, nb)
					break
				} else {
					//log.Println(h.TAG, "send size:", nb)
				}
			} else {
				log.Warnln(h.TAG, "not-send invalid msg")
			}
		case <-tickChan:
			if !h.netStat.CheckTimeout(5000) {
				log.Print2f(h.TAG, "stat[client] - %s\n", h.netStat)
			}
		case <-h.exitTick:
			log.Println(h.TAG, "exit writing")
			break exitLoop
		}
	}
	close(h.exitTick)
}

func (h *TcpHandler) sendInternal(data []byte) (error, int) {
	if nb, err := util.WriteIceTcpPacket(h.conn, data); err != nil {
		return err, -1
	} else {
		h.netStat.UpdateSend(nb)
		return nil, nb
	}
}
