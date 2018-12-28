package webrtc

import (
	"fmt"

	"github.com/PeterXu/xrtc/util"
)

const (
	kDefaultTimeout = 3000
)

/// object time

type ObjTime struct {
	utime uint64 // update time
	ctime uint64 // create time
}

func NewObjTime() *ObjTime {
	now := util.NowMs64()
	return &ObjTime{now, now}
}

func (o *ObjTime) update() {
	o.utime = util.NowMs64()
}

func (o *ObjTime) checkTimeout(timeout int) bool {
	if timeout <= 0 {
		timeout = kDefaultTimeout
	}
	now := util.NowMs64()
	return now >= o.utime+uint64(timeout)
}

/// net stat

type NetStat struct {
	sendPackets int
	sendBytes   uint64
	sendTime    uint64
	recvPackets int
	recvBytes   uint64
	recvTime    uint64
}

func NewNetStat(send, recv int) *NetStat {
	now := util.NowMs64()
	return &NetStat{1, uint64(send), now, 1, uint64(recv), now}
}

func (n *NetStat) checkTimeout(timeout int) bool {
	if timeout <= 0 {
		timeout = kDefaultTimeout
	}
	to := uint64(timeout)
	now := util.NowMs64()
	return (now >= n.sendTime+to) && (now >= n.recvTime+to)
}

func (n *NetStat) updateSend(bytes int) {
	n.sendPackets += 1
	n.sendBytes += uint64(bytes)
	n.sendTime = util.NowMs64()
}

func (n *NetStat) updateRecv(bytes int) {
	n.recvPackets += 1
	n.recvBytes += uint64(bytes)
	n.recvTime = util.NowMs64()
}

func (n NetStat) String() string {
	return fmt.Sprintf("send:%d/%d_recv:%d/%d",
		n.sendPackets, n.sendBytes, n.recvPackets, n.recvBytes)
}
