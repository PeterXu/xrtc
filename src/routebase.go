package webrtc

import (
	"github.com/PeterXu/xrtc/util"
)

type LinkInfo struct {
	toId    string
	aKey    string
	pktType RouteDataType
}

type NodeInfo struct {
	*ObjTime
	id       string
	name     string
	addrs    []string
	rtt      int
	location string
	handler  ServiceHandler
}

func NewNodeInfo() *NodeInfo {
	return &NodeInfo{ObjTime: NewObjTime()}
}

func (n *NodeInfo) mergeFrom(info *RouteDataNode) {
	if info != nil {
		n.id = info.GetId()
		n.name = info.GetName()
		for _, addr := range info.GetAddrList() {
			n.addrs = append(n.addrs, addr)
		}
		n.rtt = int(info.GetRtt())
		n.location = info.GetLocation()
	}
}

func createRoutePacket(pbType RouteDataType, fromId string, seqNo uint32) *RoutePacket {
	var pbRtt *RouteDataRtt
	if pbType == RouteDataType_RouteDataInit ||
		pbType == RouteDataType_RouteDataCheck {
		nowTime := util.NowMs64()
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

func createRouteAckPacket(recvPkt *RoutePacket, fromId string, arrivalTime uint64) *RoutePacket {
	pbType := recvPkt.GetType() + 1
	seqNo := recvPkt.GetSeqNo() + 1
	reqTime := uint64(0)
	delta := uint64(0)
	pbRtt := recvPkt.GetRtt()
	if pbRtt != nil {
		reqTime = pbRtt.GetReqTime()
		if arrivalTime > 0 {
			delta = util.NowMs64() - arrivalTime
		}
	}
	if pbRtt != nil { // reset
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
