package webrtc

import (
	"time"

	"github.com/PeterXu/xrtc/log"
	"github.com/PeterXu/xrtc/util"
	"github.com/haivision/srtgo"
)

func init() {
	srtgo.InitSRT()
	//srtgo.CleanupSRT()
}

func genSrtOptions(mode string) map[string]string {
	options := make(map[string]string)
	options["blocking"] = "0"
	options["transtype"] = "live"
	//options["pktsize"] = "1500"
	options["mode"] = mode
	return options
}

type SrtServer struct {
	TAG   string
	owner ServiceOwner
	addr  string
	sock  *srtgo.SrtSocket
	pool  *util.GoPool
}

func NewSrtServer(owner ServiceOwner, addr string) *SrtServer {
	const TAG = "[SRTS]"
	hostname, port := util.ParseHostPort(addr)
	if len(hostname) == 0 {
		hostname = "0.0.0.0" // any
	}

	options := genSrtOptions("server")
	ln := srtgo.NewSrtSocket(hostname, uint16(port), options)
	log.Println(TAG, "listen srt on:", addr, options, ln.Mode())
	if err := ln.Listen(5); err == nil {
		svr := &SrtServer{
			TAG:   TAG,
			owner: owner,
			addr:  addr,
			sock:  ln,
			pool:  util.NewGoPool(1024),
		}
		go svr.Run()
		return svr
	} else {
		log.Warnln(TAG, "Error on Listen")
		ln.Close()
		return nil
	}
}

func (s *SrtServer) Close() {
}

func (s *SrtServer) Run() {
	defer s.sock.Close()

	for {
		conn, err := s.sock.Accept()
		if err != nil {
			log.Warnln(s.TAG, "Error on Accept")
			break
		}

		handler := NewSrtHandler(s.TAG, s.owner, conn)
		s.pool.Schedule(handler.Run)
	}
}

// srt client

type SrtClient struct {
	TAG     string
	handler *SrtHandler
}

func NewSrtClient(owner ServiceOwner, addr string) *SrtClient {
	const TAG = "[SRTC]"
	hostname, port := util.ParseHostPort(addr)
	if len(hostname) == 0 {
		hostname = "127.0.0.1" // to local
	}

	options := genSrtOptions("client")
	log.Println(TAG, "Connect srt to:", addr, options)
	sock := srtgo.NewSrtSocket(hostname, uint16(port), options)
	if err := sock.Connect(); err == nil {
		cli := &SrtClient{
			TAG:     TAG,
			handler: NewSrtHandler(TAG, owner, sock),
		}
		go cli.handler.Run()
		return cli
	} else {
		log.Warnln(TAG, "Error on Connect")
		sock.Close()
		return nil
	}
}

func (s *SrtClient) Close() {
}

/// srt handler

type SrtHandler struct {
	*OneServiceHandler
	TAG   string
	owner ServiceOwner
	conn  *srtgo.SrtSocket
}

func NewSrtHandler(TAG string, owner ServiceOwner, conn *srtgo.SrtSocket) *SrtHandler {
	return &SrtHandler{
		OneServiceHandler: NewOneServiceHandler(),
		TAG:               TAG,
		owner:             owner,
		conn:              conn,
	}
}

func (h *SrtHandler) Close() {
	if h.IsReady() {
		h.conn.Close()
		h.SetClose()
	}
}

func (h *SrtHandler) Run() {
	h.SetReady()
	h.serveSrt()
}

func (h *SrtHandler) serveSrt() {
	defer h.conn.Close()

	log.Println(h.TAG, "main begin")

	go h.writeLoop()

	rbuf := make([]byte, 1024*128)
	for {
		if nret, err := h.conn.Read(rbuf[:], 0); err == nil {
			if nret > 0 {
				h.UpdateTime()
				h.netStat.UpdateRecv(nret)
				h.owner.OnRecvData(rbuf[0:nret], nil, h)
			} else {
				log.Warnln(h.TAG, "read data nothing")
			}
		} else {
			log.Warnln(h.TAG, "read data fail:", err)
			break
		}
	}

	h.SetClose()
	h.exitTick <- true
	h.owner.OnClose(h)

	log.Println(h.TAG, "main end")
}

func (h *SrtHandler) writeLoop() {
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
				log.Warnln(h.TAG, "skip invalid msg")
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

func (h *SrtHandler) sendInternal(data []byte) (error, int) {
	if nb, err := h.conn.Write(data[:], 0); err == nil {
		h.netStat.UpdateSend(nb)
		return nil, nb
	} else {
		return err, -1
	}
}
