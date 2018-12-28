package webrtc

import (
	"sync"

	log "github.com/Sirupsen/logrus"
)

type WebrtcAction uint32

const (
	WebrtcActionUnknown WebrtcAction = iota
	WebrtcActionOffer
	WebrtcActionAnswer
)

func NewWebrtcAction(data []byte, action WebrtcAction) interface{} {
	return NewHubMessage(data, nil, nil, action)
}

type Webrtc interface {
	ChanAdmin() chan interface{}
	Exit()
}

var gMutex sync.RWMutex
var gMaxHub *MaxHub
var gConfig *Config

func init() {
	log.SetLevel(log.DebugLevel)

	var fname string = "/tmp/etc/routes.yml"
	config := NewConfig()
	if !config.Load(fname) {
		log.Fatalf("[webrtc] read config failed")
		return
	}
	gConfig = config
}

func Inst() Webrtc {
	gMutex.Lock()
	defer gMutex.Unlock()
	if gMaxHub == nil {
		hub := NewMaxHub()
		go hub.Run()

		for _, cfg := range gConfig.UdpServers {
			udp := NewUdpServer(hub, cfg)
			go udp.Run()
			hub.AddServer(udp)
		}

		for _, cfg := range gConfig.TcpServers {
			tcp := NewTcpServer(hub, cfg)
			go tcp.Run()
			hub.AddServer(tcp)
		}

		for _, cfg := range gConfig.HttpServers {
			http := NewHttpServer(hub, cfg)
			go http.Run()
			hub.AddServer(http)
		}
		gMaxHub = hub
	}

	return gMaxHub
}
