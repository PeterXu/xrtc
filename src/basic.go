package webrtc

import (
	"fmt"
	"net"

	"github.com/PeterXu/xrtc/util"
)

const (
	kDefaultTimeout = 3000
)

/// obj message

type ObjMessage struct {
	data   []byte
	from   net.Addr
	to     net.Addr
	status ObjStatusType
	misc   interface{}
}

func NewObjMessage(data []byte, from net.Addr, to net.Addr, misc interface{}) *ObjMessage {
	return &ObjMessage{data, from, to, kObjStatusUnknown, misc}
}

func NewObjMessageData(data []byte, misc interface{}) *ObjMessage {
	return &ObjMessage{data, nil, nil, kObjStatusUnknown, misc}
}

func NewObjMessageStatus(status ObjStatusType, misc interface{}) *ObjMessage {
	return &ObjMessage{nil, nil, nil, status, misc}
}

/// object time

type ObjTime struct {
	utime int64 // update time
	ctime int64 // create time
}

func NewObjTime() *ObjTime {
	now := util.NowMs()
	return &ObjTime{now, now}
}

func (o *ObjTime) UpdateTime() {
	o.utime = util.NowMs()
}

func (o *ObjTime) CheckTimeout(timeout int) bool {
	if timeout <= 0 {
		timeout = kDefaultTimeout
	}
	now := util.NowMs()
	return now >= o.utime+int64(timeout)
}

/// obj status

type ObjStatusType int

const (
	kObjStatusUnknown ObjStatusType = 0
	kObjStatusInit
	kObjStatusReady
	kObjStatusFailure
	kObjStatusClose
)

type ObjStatus struct {
	objStatus ObjStatusType
}

func NewObjStatus() *ObjStatus {
	return &ObjStatus{
		objStatus: kObjStatusUnknown,
	}
}

func (o *ObjStatus) IsReady() bool {
	return o.objStatus == kObjStatusReady
}

func (o *ObjStatus) GetStatus() ObjStatusType {
	return o.objStatus
}

func (o *ObjStatus) SetStatus(status ObjStatusType) {
	o.objStatus = status
}

func (o *ObjStatus) SetInit() {
	o.SetStatus(kObjStatusInit)
}

func (o *ObjStatus) SetReady() {
	o.SetStatus(kObjStatusReady)
}

func (o *ObjStatus) SetClose() {
	o.SetStatus(kObjStatusClose)
}

/// net stat

type NetStat struct {
	sendPackets int
	sendBytes   uint64
	sendTime    int64
	recvPackets int
	recvBytes   uint64
	recvTime    int64
}

func NewNetStat(send, recv int) *NetStat {
	now := util.NowMs()
	return &NetStat{1, uint64(send), now, 1, uint64(recv), now}
}

func (n *NetStat) CheckTimeout(timeout int) bool {
	if timeout <= 0 {
		timeout = kDefaultTimeout
	}
	to := int64(timeout)
	now := util.NowMs()
	return (now >= n.sendTime+to) && (now >= n.recvTime+to)
}

func (n *NetStat) UpdateSend(bytes int) {
	n.sendPackets += 1
	n.sendBytes += uint64(bytes)
	n.sendTime = util.NowMs()
}

func (n *NetStat) UpdateRecv(bytes int) {
	n.recvPackets += 1
	n.recvBytes += uint64(bytes)
	n.recvTime = util.NowMs()
}

func (n NetStat) String() string {
	return fmt.Sprintf("send:%d/%d_recv:%d/%d",
		n.sendPackets, n.sendBytes, n.recvPackets, n.recvBytes)
}

/// net handler

type NetHandler struct {
	netStat  *NetStat
	chanFeed chan interface{}
	exitTick chan bool
}

func NewNetHandler() *NetHandler {
	return &NetHandler{
		netStat:  NewNetStat(0, 0),
		chanFeed: make(chan interface{}, 100),
		exitTick: make(chan bool),
	}
}

/// webrtc ice info

// byRoute: true, means that client -> proxy .. proxy -> server
// byRoute: false, means that client -> proxy -> server
type WebrtcIce struct {
	OfferIce   util.SdpIceAttr
	AnswerIce  util.SdpIceAttr
	Candidates []string
	byRoute    bool
}
