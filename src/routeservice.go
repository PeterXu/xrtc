package webrtc

import (
	"errors"
	"net"
	"time"

	proto "github.com/golang/protobuf/proto"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

/// Route Service

type RouteService struct {
	*OneService
	*NetHandler

	nodes     map[string]*NodeInfo // should keep <= 30
	handlers  map[string]ServiceHandler
	chanRoute chan interface{}

	svr  *SrtServer
	clis map[string]*SrtClient
}

func NewRouteService(hub *MaxHub, cfg *ModConfig) *RouteService {
	const TAG = "[ROUTE]"
	route := &RouteService{
		OneService: NewOneService(TAG, hub, cfg),
		NetHandler: NewNetHandler(),
		nodes:      make(map[string]*NodeInfo),
		handlers:   make(map[string]ServiceHandler),
		chanRoute:  make(chan interface{}),
		clis:       make(map[string]*SrtClient),
	}
	for _, addr := range cfg.Addrs {
		proto, hostport := util.ParseUri(addr)
		switch proto {
		case "srt":
			route.svr = NewSrtServer(route, hostport)
		default:
			return nil
		}
	}
	go route.Run()
	return route
}

func (s *RouteService) Close() {
	s.exitTick <- true
}

func (s *RouteService) MyId() string {
	return s.SysId()
}

func (s *RouteService) MyNode() *RouteDataNode {
	myId := s.SysId()
	myName := s.SysName()
	myLocation := s.RouteParams().Location.String()
	return &RouteDataNode{
		Id:       &myId,
		Name:     &myName,
		AddrList: nil,
		Load:     nil,
		Capacity: &s.RouteParams().Capacity,
		Location: &myLocation,
	}
}

// data flow:
//      a. webrtc client -> current xrtc -> other xrtc: rtp/rtcp/datachannel
//      b. self -> current xrtc -> other xrtc: route-msg
func (s *RouteService) SendData(msg interface{}) {
	s.chanFeed <- msg
}

func (s *RouteService) Run() {
	tickChan := time.NewTicker(time.Second * 1).C

exitLoop:
	for {
		select {
		case msg, ok := <-s.chanFeed:
			if !ok {
				log.Warnln(s.TAG, "go-channel recv closed")
				break exitLoop
			}

			umsg, ok := msg.(*ObjMessage)
			if !ok {
				log.Warnln(s.TAG, "invalid recv msg")
				break
			}

			uinfo, ok := umsg.misc.(*LinkInfo)
			if !ok {
				log.Warnln(s.TAG, "invalid link info")
				break
			}
			handler := s.getHandlerById(uinfo.toId)
			if handler == nil {
				log.Warnln(s.TAG, "no recv handler for id="+uinfo.toId)
				break
			}
			packet := createRoutePacket(uinfo.pktType, s.MyId(), handler.GetOneSeqNo())
			packet.ToId = &uinfo.toId
			packet.Akey = &uinfo.aKey
			packet.Payload = umsg.data
			if data, err := proto.Marshal(packet); err == nil {
				handler.SendData(data, nil)
			}
		case msg, ok := <-s.chanRoute:
			if !ok {
				log.Warnln(s.TAG, "go-channel route closed")
				break exitLoop
			}

			umsg, ok := msg.(*ObjMessage)
			if !ok {
				log.Warnln(s.TAG, "invalid route msg")
				break
			}

			handler, ok := umsg.misc.(ServiceHandler)
			if !ok {
				log.Warnln(s.TAG, "invalid route handler")
				break
			}
			if umsg.status == kObjStatusClose {
				s.closeHandler(nil, handler)
			} else {
				err := s.processPacket(umsg.data, handler)
				if err != nil {
					log.Warnln(s.TAG, err)
				}
			}
		case <-tickChan:
			s.checkStatus()
		case <-s.exitTick:
			log.Println(s.TAG, "exit")
			break exitLoop
		}
	}

	close(s.exitTick)
}

// The data flow when from remote proxy (xrtc):
//      a. other xrtc -> current xrtc -> self: route-msg
//      b. other xrtc -> current xrtc -> another xrtc: rtp/rtcp/datachannel
//      c. other xrtc -> current xrtc -> webrtc client: rtp/rtcp/datachannel
func (s *RouteService) OnRecvData(data []byte, raddr net.Addr, from ServiceHandler) {
	buff := util.Clone(data)
	s.chanRoute <- NewObjMessage(buff, raddr, nil, from)
}

func (s *RouteService) OnClose(from ServiceHandler) {
	s.chanRoute <- NewObjMessageStatus(kObjStatusClose, from)
}

func (s *RouteService) processPacket(data []byte, from ServiceHandler) error {
	packet := &RoutePacket{}
	if err := proto.Unmarshal(data, packet); err != nil {
		return err
	}

	ch := from.GetFeedChan()
	fromId := packet.GetFromId()
	toId := packet.GetToId()

	pbType := packet.GetType()
	switch pbType {
	case RouteDataType_RouteDataNone:
		return errors.New("invalid route type")
	case RouteDataType_RouteDataRtp, RouteDataType_RouteDataRtcp, RouteDataType_RouteDataChannel:
		if len(fromId) == 0 || len(toId) == 0 {
			return errors.New("invalid id in route packet(rtp/rtcp/datachannel)")
		}
		if toId == s.MyId() {
			// to self
			buff := util.Clone(data)
			s.GetDataInChan() <- NewObjMessageData(buff, ch)
		} else {
			// forward
			handler := s.selectOneHandler(packet)
			if handler != nil {
				handler.SendData(data, nil)
			}
		}
	case RouteDataType_RouteDataInit, RouteDataType_RouteDataInitAck,
		RouteDataType_RouteDataCheck, RouteDataType_RouteDataCheckAck:
		if len(fromId) == 0 {
			return errors.New("wrong id in route init/ack")
		}
		node, ok := s.nodes[fromId]
		if !ok {
			if pbType == RouteDataType_RouteDataInit || pbType == RouteDataType_RouteDataInitAck {
				node = NewNodeInfo()
				s.nodes[fromId] = node
			}
		}
		if node == nil {
			return errors.New("no route node for id=" + fromId)
		}
		node.UpdateTime()
		node.handler = from
		node.mergeFrom(packet.GetNode())

		if pbType == RouteDataType_RouteDataInit || pbType == RouteDataType_RouteDataCheck {
			respPkt := createRouteAckPacket(packet, s.MyId(), 0)
			if respData, err := proto.Marshal(respPkt); err == nil {
				from.SendData(respData, nil)
			}
		} else {
			pbRtt := packet.GetRtt()
			if pbRtt != nil {
				rttVal := int(util.NowMs64() - pbRtt.GetReqTime() - pbRtt.GetDelta())
				node.rtt = rttVal
			}
		}
	default:
		return errors.New("unknown route type")
	}

	return nil
}

func (s *RouteService) checkStatus() {
	// remove timeout handler
	for _, node := range s.nodes {
		if node.handler != nil && node.handler.CheckTimeout(5*1000) {
			node.handler.Close()
			s.closeHandler(node, node.handler)
		}
	}

	// send check peridocally
	for _, node := range s.nodes {
		if node.handler != nil && node.CheckTimeout(3000) {
			packet := createRoutePacket(RouteDataType_RouteDataCheck, s.MyId(), node.handler.GetOneSeqNo())
			if data, err := proto.Marshal(packet); err == nil {
				node.handler.SendData(data, nil)
			}
		}
	}

	// try to connect initial remotes
	for _, addr := range s.RouteParams().InitAddrs {
		break
		if cli, ok := s.clis[addr]; !ok {
			if cli = NewSrtClient(s, addr); cli != nil {
				packet := createRoutePacket(RouteDataType_RouteDataInit, s.MyId(), cli.handler.GetOneSeqNo())
				packet.Node = s.MyNode()
				if data, err := proto.Marshal(packet); err == nil {
					cli.handler.SendData(data, nil)
				}
				s.clis[addr] = cli
			} else {
				log.Warnln(s.TAG, "fail to connect with "+addr)
			}
		}
	}
}

func (s *RouteService) closeHandler(node *NodeInfo, handler ServiceHandler) {
	if node == nil {
		for _, val := range s.nodes {
			if val.handler == handler {
				node = val
				break
			}
		}
	}
	if node != nil {
		delete(s.handlers, node.id)
		for addr, cli := range s.clis {
			if cli.handler == node.handler {
				delete(s.clis, addr)
				break
			}
		}
		node.handler = nil
	}
}

func (s *RouteService) getHandlerById(id string) ServiceHandler {
	if val, ok := s.nodes[id]; ok {
		return val.handler
	}
	return nil
}

func (s *RouteService) selectOneHandler(pkt *RoutePacket) ServiceHandler {
	ttl := pkt.GetTtl()
	if ttl > 0 {
		*pkt.Ttl = ttl - 1
	}

	key := pkt.GetAkey()
	if ptr, ok := s.handlers[key]; ok {
		return ptr
	}

	var handler ServiceHandler
	if ttl == 0 {
		handler = s.getHandlerById(pkt.GetToId())
	}
	if handler == nil {
		rtt := 0xffff
		for _, val := range s.nodes {
			if val.handler != nil {
				if handler == nil {
					handler = val.handler
				} else if val.rtt > 0 && val.rtt < rtt {
					rtt = val.rtt
					handler = val.handler
				}
			}
		}
	}
	if handler != nil {
		s.handlers[key] = handler
	}
	return handler
}

func (s *RouteService) checkRouteLink(srcLoc, dstLoc *GeoLocation) bool {
	return false
}
