package webrtc

import (
	"github.com/PeterXu/xrtc/util"
)

const (
	kMaxRouteNodeNumber = 30
)

type LinkInfo struct {
	toId    string
	aKey    string
	pktType RouteDataType
}

type RoutePeer struct {
	id       string
	name     string
	addrs    []string
	capacity uint32
	location *GeoLocation

	*ObjTime
	rtt     uint32
	handler ServiceHandler
}

func NewRoutePeer() *RoutePeer {
	return &RoutePeer{
		ObjTime:  NewObjTime(),
		location: NewGeoLocation(),
	}
}

func (p *RoutePeer) mergeFrom(node *RouteDataNode) {
	if node != nil {
		p.id = node.GetId()
		p.name = node.GetName()
		p.addrs = util.CloneArray(node.GetAddrs())
		p.capacity = node.GetCapacity()
		p.location.MergeFrom(node.GetLocation())
	}
}

func (p *RoutePeer) genRouteNode() *RouteDataNode {
	loc := p.location.String()
	return &RouteDataNode{
		Id:       &p.id,
		Name:     &p.name,
		Addrs:    util.CloneArray(p.addrs),
		Capacity: &p.capacity,
		Location: &loc,
	}
}

/// Tools for RoutePacket

func (r RouteDataRtt) getResult() int {
	return int(util.NowMs() - r.GetReqTime() - r.GetDelta())
}

func (r RoutePacket) createAckPacket(fromId string, arrivalTime int64) *RoutePacket {
	pbType := r.GetType() + 1
	seqNo := r.GetSeqNo() + 1
	pbRtt := r.GetRtt()
	if pbRtt != nil {
		reqTime := pbRtt.GetReqTime()
		delta := int64(0)
		if arrivalTime > 0 {
			delta = util.NowMs() - arrivalTime
		}
		pbRtt = &RouteDataRtt{
			ReqTime: &reqTime,
			Delta:   &delta,
		}
	}
	return &RoutePacket{
		Type:   &pbType,
		FromId: &fromId,
		SeqNo:  &seqNo,
		Rtt:    pbRtt,
	}
}

func createRoutePacket(pbType RouteDataType, fromId string, seqNo uint32) *RoutePacket {
	var pbRtt *RouteDataRtt
	if pbType == RouteDataType_RouteDataInit ||
		pbType == RouteDataType_RouteDataCheck {
		nowTime := util.NowMs()
		pbRtt = &RouteDataRtt{
			ReqTime: &nowTime,
		}
	}
	return &RoutePacket{
		Type:   &pbType,
		FromId: &fromId,
		SeqNo:  &seqNo,
		Rtt:    pbRtt,
	}
}
