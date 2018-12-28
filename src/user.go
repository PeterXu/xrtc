package webrtc

import (
	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

type User struct {
	TAG string

	iceTcp      bool                   // connect with webrtc server by tcp/udp
	iceDirect   bool                   // forward ice stun between outer and inner
	connections map[string]*Connection // outer client connections
	chanSend    chan interface{}       // data to inner(server)
	service     *Service               // inner webrtc server

	leave      bool
	activeConn *Connection // active conn
	sendIce    SdpIceInfo
	recvIce    SdpIceInfo

	utime uint64 // update time
	ctime uint64 // create time
}

func NewUser(iceTcp, iceDirect bool) *User {
	now := util.NowMs64()
	return &User{
		TAG:         "[USER]",
		iceTcp:      iceTcp,
		iceDirect:   iceDirect,
		connections: make(map[string]*Connection),
		chanSend:    make(chan interface{}, 100),
		utime:       now,
		ctime:       now,
	}
}

func (u *User) getIceKey() string {
	return u.recvIce.Ufrag + ":" + u.sendIce.Ufrag
}

func (u *User) setIceInfo(offerIce, answerIce *SdpIceInfo, candidates []string) bool {
	log.Println(u.TAG, "set ice info:", offerIce, answerIce)
	u.recvIce = *offerIce  // recv from offer(client -> proxy)
	u.sendIce = *answerIce // send from answer(proxy -> server)
	return u.startService(candidates)
}

func (u *User) getSendIce() SdpIceInfo {
	return u.sendIce // from answer
}

func (u *User) getRecvIce() SdpIceInfo {
	return u.recvIce // from offer
}

func (u *User) isIceTcp() bool {
	return u.iceTcp
}

func (u *User) isIceDirect() bool {
	return u.iceDirect
}

func (u *User) addConnection(conn *Connection) {
	if conn != nil && conn.getAddr() != nil {
		u.connections[util.NetAddrString(conn.getAddr())] = conn
		if u.activeConn == nil {
			u.activeConn = conn
		}
	} else {
		log.Warnln(u.TAG, "no conn or addr")
	}
}

func (u *User) delConnection(conn *Connection) {
	if conn != nil {
		delete(u.connections, util.NetAddrString(conn.getAddr()))
	}
}

func (u *User) sendToInner(conn *Connection, data []byte) {
	if u.leave {
		return
	}
	u.activeConn = conn
	u.chanSend <- data
}

func (u *User) sendToOuter(data []byte) {
	if u.leave {
		return
	}

	if u.activeConn == nil {
		for k, v := range u.connections {
			if v.isReady() {
				u.activeConn = v
				log.Println(u.TAG, "choose active conn, id=", k)
				break
			}
		}
	}

	if u.activeConn == nil {
		log.Warnln(u.TAG, "no active connection")
		return
	}

	u.activeConn.sendData(data)
}

func (u *User) isTimeout() bool {
	if len(u.connections) == 0 {
		return true
	}
	return false
}

func (u *User) dispose() {
	log.Println(u.TAG, "dispose, connection size=", len(u.connections))
	u.leave = true
	if u.service != nil {
		u.service.dispose()
	}
	if len(u.connections) > 0 {
		u.connections = make(map[string]*Connection)
	}
}

func (u *User) onServiceClose() {
	u.leave = true
}

func (u *User) startService(candidates []string) bool {
	if u.service != nil {
		return true
	}

	sice := u.getRecvIce()
	rice := u.getSendIce()
	remoteSdp := genServiceSdp(rice.Ufrag, rice.Pwd, candidates)
	log.Println(u.TAG, "start service, send/recvIce=", sice, rice, remoteSdp)

	bret := false
	u.service = NewService(u, u.chanSend)
	if u.service.Init(sice.Ufrag, sice.Pwd, remoteSdp) {
		bret = u.service.Start()
	}
	log.Println(u.TAG, "start service ret=", bret)
	return bret
}
