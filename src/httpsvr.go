package webrtc

import (
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/PeterXu/xrtc/log"
	"github.com/PeterXu/xrtc/util"
)

type HttpServer struct {
	TAG   string
	owner ServiceOwner
	addr  string
	ln    net.Listener
	pool  *util.GoPool
}

// http server (http/https/ws/wss)
func NewHttpServer(owner ServiceOwner, addr string) *HttpServer {
	const TAG = "[HTTP]"
	log.Println(TAG, "listen http on:", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(TAG, "listen http error=", err)
		return nil
	}
	svr := &HttpServer{
		TAG:   TAG,
		owner: owner,
		addr:  addr,
		ln:    l,
		pool:  util.NewGoPool(1024),
	}
	go svr.Run()
	return svr
}

func (s *HttpServer) Close() {
	//s.ln.Close()
}

func (s *HttpServer) Run() {
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
			log.Print2f(s.TAG, "accept error: %v; retrying after %v", err, tempDelay)
			time.Sleep(tempDelay)
			continue
		}
		tempDelay = 0

		handler := NewHttpHandler(s.TAG, s.owner, conn)
		s.pool.Schedule(handler.Run)
	}
}

/// http handler

type HttpHandler struct {
	TAG   string
	owner ServiceOwner
	conn  *util.NetConn
}

func NewHttpHandler(TAG string, owner ServiceOwner, conn net.Conn) *HttpHandler {
	return &HttpHandler{
		TAG:   TAG,
		owner: owner,
		conn:  util.NewNetConn(conn),
	}
}

func (h *HttpHandler) Run() {
	if !h.serveHTTP() {
		// Close here only when failed
		h.conn.Close()
	}
}

func (h *HttpHandler) serveHTTP() bool {
	kClientHello := util.SslClientHello
	kClientLen := len(util.SslClientHello)
	kServerHello := util.SslServerHello

	data, err := h.conn.Peek(kClientLen)
	if len(data) < 3 {
		log.Print2f(h.TAG, "no enough data, err: %v", err)
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

	isSsl := false
	if len(data) >= kClientLen && bytes.Compare(data[0:kClientLen], kClientHello) == 0 {
		isSsl = true
	} else if bytes.Compare(prefix, kServerHello[0:prelen]) != 0 {
		isSsl = false
	} else {
		isSsl = true
	}

	if isSsl {
		//log.Println(s.TAG, "setup tls key/cert for", h.conn.RemoteAddr())
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

		// set new conn(tls)
		h.conn = util.NewNetConn(conn2)
	}

	// now it is plain conn for tcp/http
	//log.Println(h.TAG, "setup http/https for", h.conn.RemoteAddr())
	http.Serve(
		NewHttpListener(h.TAG, h.conn),
		NewHttpServeHandler(h.owner.Name(), h.owner.RestParams()),
	)
	//log.Println(h.TAG, "setup success")
	return true
}

func NewHttpListener(TAG string, c *util.NetConn) *HttpListener {
	return &HttpListener{TAG, c, false, false}
}

type HttpListener struct {
	TAG     string
	conn    *util.NetConn
	used    bool
	closing bool
}

func (l *HttpListener) Accept() (net.Conn, error) {
	//log.Println(l.TAG, "HttpListener Accept.., ", l.used)
	if !l.used {
		l.used = true
		return l.conn, nil
	} else {
		return nil, errors.New(l.TAG + ":HttpListener Quit")
	}
}

func (l *HttpListener) Close() error {
	//log.Println(l.TAG, "HttpListener Close..,", l.closing)
	if l.closing {
		l.conn.Close()
	} else {
	}
	return nil
}

func (l *HttpListener) Addr() net.Addr {
	return l.conn.RemoteAddr()
}
