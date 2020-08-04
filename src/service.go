package webrtc

type OneService interface {
	Run()
	Close()
	Params() *NetParams
}

type OneServer struct {
	TAG    string
	hub    *MaxHub
	config *NetConfig
}

func (s *OneServer) Init(TAG string, hub *MaxHub, cfg *NetConfig) {
	s.TAG = TAG
	s.hub = hub
	s.config = cfg
}

func (s *OneServer) Close() {
	// nop
}

func (s *OneServer) Params() *NetParams {
	return &s.config.Net
}

func (s *OneServer) GetSslFile() (string, string) {
	return s.config.Net.TlsCrtFile, s.config.Net.TlsKeyFile
}

// client/rourter -> in here -> hub-proces
func (s *OneServer) GetDataInChan() chan interface{} {
	switch s.config.Proto {
	case "udp", "tcp":
		return s.hub.ChanRecvFromClient()
	case "srt":
		return s.hub.ChanRecvFromRouter()
	}
	return nil
}
