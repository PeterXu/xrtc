package webrtc

import (
	"net"
)

type Service interface {
	Close()
	SysId() string
	SysName() string
	Candidates() []string
}

type ServiceOwner interface {
	Service
	Config() *ModConfig
	Name() string
	RouteParams() *RouteNetParams
	IceParams() *IceNetParams
	RestParams() *RestNetParams
	GetSslFile() (string, string)
	OnRecvData(data []byte, raddr net.Addr, from ServiceHandler)
	OnClose(from ServiceHandler)
}

type ServiceHandler interface {
	Close()
	GetFeedChan() chan interface{}
	GetOneSeqNo() uint32
	SendData(data []byte, to net.Addr)
	CheckTimeout(timeout int) bool
}

/// one service

type OneService struct {
	TAG string
	hub *MaxHub
	cfg *ModConfig
}

func NewOneService(TAG string, hub *MaxHub, cfg *ModConfig) *OneService {
	return &OneService{
		TAG: TAG,
		hub: hub,
		cfg: cfg,
	}
}

//> Service

func (s *OneService) Close() {
	// nop
}

func (s *OneService) SysId() string {
	if s.cfg.Common != nil {
		return s.cfg.Common.Id
	} else {
		return ""
	}
}

func (s *OneService) SysName() string {
	if s.cfg.Common != nil {
		return s.cfg.Common.Name
	} else {
		return ""
	}
}

func (s *OneService) Candidates() []string {
	if s.cfg.Ice != nil {
		return s.cfg.Ice.Candidates
	} else {
		return nil
	}
}

//> Service Owner

func (s *OneService) Config() *ModConfig {
	return s.cfg
}

func (s *OneService) Name() string {
	return s.cfg.Name
}

func (s *OneService) RouteParams() *RouteNetParams {
	return s.cfg.Route
}

func (s *OneService) IceParams() *IceNetParams {
	return s.cfg.Ice
}

func (s *OneService) RestParams() *RestNetParams {
	return s.cfg.Rest
}

func (s *OneService) GetSslFile() (string, string) {
	if s.cfg.Common != nil {
		return s.cfg.Common.CrtFile, s.cfg.Common.KeyFile
	} else {
		return "", ""
	}
}

func (s *OneService) OnRecvData(data []byte, raddr net.Addr, from ServiceHandler) {
	// nop
}

func (s *OneService) OnClose(from ServiceHandler) {
	// nop
}

// client/rourter -> in here -> hub-proces
func (s *OneService) GetDataInChan() chan interface{} {
	return GetHubDataInChan(s.hub, s.cfg.Mod)
}

/// one service handler

type OneServiceHandler struct {
	*ObjTime
	*ObjStatus
	*NetHandler
	seqNo uint32
}

func NewOneServiceHandler() *OneServiceHandler {
	return &OneServiceHandler{
		ObjTime:    NewObjTime(),
		ObjStatus:  NewObjStatus(),
		NetHandler: NewNetHandler(),
		seqNo:      5000,
	}
}

func (h *OneServiceHandler) Close() {
	// nop
}

func (h *OneServiceHandler) GetFeedChan() chan interface{} {
	return h.chanFeed
}

func (h *OneServiceHandler) GetOneSeqNo() uint32 {
	h.seqNo += 1
	return h.seqNo
}

func (h *OneServiceHandler) SendData(data []byte, to net.Addr) {
	h.chanFeed <- NewObjMessage(data, nil, to, nil)
}

/// others

func GetHubDataInChan(hub *MaxHub, mod string) chan interface{} {
	switch mod {
	case "ice":
		return hub.ChanRecvFromClient()
	case "route":
		return hub.ChanRecvFromRouter()
	default:
		return nil
	}
}

func CreateService(hub *MaxHub, cfg *ModConfig) Service {
	switch cfg.Mod {
	case "route":
		return NewRouteService(hub, cfg)
	case "ice":
		return NewIceService(hub, cfg)
	case "rest":
		return NewRestService(hub, cfg)
	default:
		return nil
	}
}
