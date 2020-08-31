package webrtc

import (
	"net"

	"github.com/PeterXu/xrtc/log"
	"github.com/PeterXu/xrtc/util"
)

type IceService struct {
	*OneService
	udpsvr *UdpServer
	tcpsvr *TcpServer
}

func NewIceService(hub *MaxHub, cfg *ModConfig) *IceService {
	const TAG = "[ICE]"
	ice := &IceService{
		OneService: NewOneService(TAG, hub, cfg),
	}
	for _, addr := range cfg.Addrs {
		proto, hostport := util.ParseUri(addr)
		switch proto {
		case "udp":
			ice.udpsvr = NewUdpServer(ice, hostport)
		case "tcp":
			ice.tcpsvr = NewTcpServer(ice, hostport)
		default:
			return nil
		}
	}
	return ice
}

func (s *IceService) Close() {
	log.Println(s.TAG, "Close")
	if s.udpsvr != nil {
		s.udpsvr.Close()
	}
	if s.tcpsvr != nil {
		s.tcpsvr.Close()
	}
}

func (s *IceService) OnRecvData(data []byte, raddr net.Addr, from ServiceHandler) {
	ch := from.GetFeedChan()
	buff := util.Clone(data)
	s.GetDataInChan() <- NewObjMessage(buff, raddr, nil, ch)
}

func (s *IceService) OnClose(from ServiceHandler) {
}
