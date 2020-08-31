package webrtc

import (
	"github.com/PeterXu/xrtc/util"
)

type WebRTC interface {
	Cache() *Cache
	Candidates() []string // proxy candidates
	Close()
}

func DefaultConfig() string {
	return kDefaultConfig
}

func NewWebRTC(fname string) WebRTC {
	config := LoadConfig(fname)
	if config == nil {
		return nil
	}
	if config.common != nil {
		initGeoLite(config.common.GeoLiteFile)
	}

	hub := NewMaxHub()
	util.Sleep(100)
	for _, cfg := range config.configs {
		hub.AddService(CreateService(hub, cfg))
		util.Sleep(100)
	}
	ggHub = hub
	return hub
}

var ggHub *MaxHub

func Inst() WebRTC {
	return ggHub
}
