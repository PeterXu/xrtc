package webrtc

import (
	//"net"
	//"time"

	"github.com/PeterXu/xrtc/util"
	//log "github.com/PeterXu/xrtc/util"
)

type RestService struct {
	*OneService
	httpsvr *HttpServer
}

func NewRestService(hub *MaxHub, cfg *ModConfig) *RestService {
	const TAG = "[REST]"
	rest := &RestService{
		OneService: NewOneService(TAG, hub, cfg),
	}
	for _, addr := range cfg.Addrs {
		proto, hostport := util.ParseUri(addr)
		switch proto {
		case "http":
			rest.httpsvr = NewHttpServer(rest, hostport)
		default:
			return nil
		}
	}
	return rest
}

func (s *RestService) Close() {
	if s.httpsvr != nil {
		s.httpsvr.Close()
	}
}
