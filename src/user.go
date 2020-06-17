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
	iceAgent    *IceAgent              // inner ice agent (connect to media server)

	leave      bool
	activeConn *Connection // active conn
	sendIce    SdpIceJson
	recvIce    SdpIceJson

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

func (u *User) setIceInfo(offerIce, answerIce *SdpIceJson, candidates []string) bool {
	log.Println(u.TAG, "set ice info:", offerIce, answerIce)
	u.recvIce = *offerIce  // recv from offer(client -> proxy)
	u.sendIce = *answerIce // send from answer(proxy -> server)
	return u.startAgent(candidates)
}

func (u *User) getSendIce() SdpIceJson {
	return u.sendIce // from answer
}

func (u *User) getRecvIce() SdpIceJson {
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
	if u.iceAgent != nil {
		u.iceAgent.dispose()
	}
	if len(u.connections) > 0 {
		u.connections = make(map[string]*Connection)
	}
}

func (u *User) onAgentClose() {
	u.leave = true
}

func (u *User) startAgent(candidates []string) bool {
	if u.iceAgent != nil {
		return true
	}

	sice := u.getRecvIce()
	rice := u.getSendIce()
	remoteSdp := genIceAgentSdp(rice.Ufrag, rice.Pwd, candidates)
	log.Println(u.TAG, "start iceAgent, send/recvIce=", sice, rice, remoteSdp)

	bret := false
	u.iceAgent = NewIceAgent(u, u.chanSend)
	if u.iceAgent.Init(sice.Ufrag, sice.Pwd, remoteSdp) {
		bret = u.iceAgent.Start()
	}
	log.Println(u.TAG, "start agent ret=", bret)
	return bret
}
