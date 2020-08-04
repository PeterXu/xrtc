package webrtc

import (
	"sync"

	log "github.com/PeterXu/xrtc/util"
)

type Webrtc interface {
	Cache() *Cache
	Candidates() []string // proxy candidates
	Close()
}

// gloabl variables
var gMutex sync.RWMutex
var gMaxHub *MaxHub

// load config parameters.
func loadConfig(fname string) *Config {
	config := NewConfig()
	if !config.Load(fname) {
		log.Fatal("read config failed")
		return nil
	}
	return config
}

func createService(hub *MaxHub, cfg *NetConfig) OneService {
	switch cfg.Proto {
	case "srt":
		return NewSrtServer(hub, cfg)
	case "udp":
		return NewUdpServer(hub, cfg)
	case "tcp":
		return NewTcpServer(hub, cfg)
	case "http":
		return NewHttpServer(hub, cfg)
	default:
		return nil
	}
}

// Inst the global entry function.
func Inst() Webrtc {
	gMutex.Lock()
	defer gMutex.Unlock()
	if gMaxHub == nil {
		config := loadConfig(kDefaultConfig)
		if config != nil {
			hub := NewMaxHub()
			for _, cfg := range config.Services {
				hub.AddService(createService(hub, cfg))
			}
			gMaxHub = hub
		}
	}
	return gMaxHub
}
