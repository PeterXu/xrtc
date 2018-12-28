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

func createServer(hub *MaxHub, cfg *NetConfig) OneServer {
	switch cfg.Proto {
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

// start servers from config.
func startServers(hub *MaxHub, config *Config) {
	for _, cfg := range config.Servers {
		hub.AddServer(createServer(hub, cfg))
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
			startServers(hub, config)
			gMaxHub = hub
		}
	}
	return gMaxHub
}
