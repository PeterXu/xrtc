package webrtc

import (
	"time"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
	"github.com/haivision/srtgo"
)

type SrtServer struct {
	OneServer

	sock *srtgo.SrtSocket
	pool *util.GoPool
}

func NewSrtServer(hub *MaxHub, cfg *NetConfig) *SrtServer {
	const TAG = "[SRT]"

	hostname, port := util.ParseHostPort(cfg.Net.Addr)
	if len(hostname) == 0 {
		hostname = "0.0.0.0"
	}

	options := make(map[string]string)
	options["blocking"] = "1"
	options["transtype"] = "live"

	log.Println(TAG, "listen srt on:", cfg.Net.Addr)
	ln := srtgo.NewSrtSocket(hostname, uint16(port), options)
	err := ln.Listen(5)
	if err != nil {
		log.Warnln(TAG, "Error on Listen")
		return nil
	} else {
		svr := &SrtServer{
			sock: ln,
			pool: util.NewGoPool(1024),
		}
		svr.Init(TAG, hub, cfg)
		go svr.Run()
		return svr
	}
}

func (s *SrtServer) Run() {
	defer s.sock.Close()

	for {
		conn, err := s.sock.Accept()
		if err != nil {
			log.Warnln(s.TAG, "Error on Accept")
			break
		}

		handler := NewSrtHandler(s, conn)
		s.pool.Schedule(handler.Run)
	}
}

/// srt handler

type SrtHandler struct {
	TAG      string
	svr      *SrtServer
	conn     *srtgo.SrtSocket
	stat     *NetStat
	chanRecv chan interface{}
	exitTick chan bool
}

func NewSrtHandler(svr *SrtServer, conn *srtgo.SrtSocket) *SrtHandler {
	return &SrtHandler{
		TAG:      svr.TAG,
		svr:      svr,
		conn:     conn,
		stat:     NewNetStat(0, 0),
		chanRecv: make(chan interface{}, 100),
		exitTick: make(chan bool),
	}
}

func (h *SrtHandler) Run() {
	h.ServeSrt()
}

func (h *SrtHandler) ServeSrt() {
	defer h.conn.Close()

	log.Println(h.TAG, "main begin")

	go h.writeLoop()

	inChan := h.svr.GetDataInChan()

	rbuf := make([]byte, 1024*128)
	for {
		if nret, err := h.conn.Read(rbuf[:], 0); err == nil {
			if nret > 0 {
				h.stat.updateRecv(nret)
				data := make([]byte, nret)
				copy(data, rbuf[0:nret])
				inChan <- NewHubMessage(data, nil, nil, h.chanRecv)
			} else {
				log.Warnln(h.TAG, "read data nothing")
			}
		} else {
			log.Warnln(h.TAG, "read data fail:", err)
			break
		}
	}

	h.exitTick <- true

	log.Println(h.TAG, "main end")
}

func (h *SrtHandler) writeLoop() {
	tickChan := time.NewTicker(time.Second * 10).C

	for {
		select {
		case msg, ok := <-h.chanRecv:
			if !ok {
				log.Println(h.TAG, "close channel")
				return
			}

			if umsg, ok := msg.(*HubMessage); ok {
				if err, nb := h.Send(umsg.data); err != nil {
					log.Warnln(h.TAG, "send err:", err, nb)
					return
				} else {
					//log.Println(h.TAG, "send size:", nb)
				}
			} else {
				log.Warnln(h.TAG, "skip invalid msg")
			}
		case <-tickChan:
			if !h.stat.checkTimeout(5000) {
				log.Print2f(h.TAG, "stat[client] - %s\n", h.stat)
			}
		case <-h.exitTick:
			log.Println(h.TAG, "exit writing")
			return
		}
	}
}

func (h *SrtHandler) Send(data []byte) (error, int) {
	if nb, err := h.conn.Write(data[:], 0); err != nil {
		return err, -1
	} else {
		h.stat.updateSend(nb)
		return nil, nb
	}
}
