package webrtc

import (
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

type HttpServer struct {
	TAG    string
	hub    *MaxHub
	ln     net.Listener
	config *NetConfig
	pool   *util.GoPool
}

// http server (http/https/ws/wss)
func NewHttpServer(hub *MaxHub, cfg *NetConfig) *HttpServer {
	const TAG = "[HTTP]"
	//addr := fmt.Sprintf(":%d", port)
	addr := cfg.Net.Addr
	log.Println(TAG, "listen on: ", addr)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(TAG, "listen error=", err)
		return nil
	}
	svr := &HttpServer{
		TAG:    TAG,
		hub:    hub,
		ln:     l,
		config: cfg,
		pool:   util.NewGoPool(1024),
	}
	go svr.Run()
	return svr
}

func (s *HttpServer) GetSslFile() (string, string) {
	return s.config.Net.TlsCrtFile, s.config.Net.TlsKeyFile
}

func (s *HttpServer) Params() *NetParams {
	return &s.config.Net
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
			return
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

		handler := NewHttpHandler(s, conn)
		s.pool.Schedule(handler.Run)
	}
}

func (s *HttpServer) Close() {
	//s.ln.Close()
}

type HttpHandler struct {
	TAG  string
	svr  *HttpServer
	conn *util.NetConn
}

func NewHttpHandler(svr *HttpServer, conn net.Conn) *HttpHandler {
	return &HttpHandler{
		TAG:  svr.TAG,
		svr:  svr,
		conn: util.NewNetConn(conn),
	}
}

func (h *HttpHandler) Run() {
	if !h.Process() {
		// Close here only when failed
		h.conn.Close()
	}
}

func (h *HttpHandler) Process() bool {
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
		cer, err := tls.LoadX509KeyPair(h.svr.GetSslFile())
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
		NewHttpServeHandler(h.svr.config.Name, &h.svr.config.Http),
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
