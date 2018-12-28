package webrtc

import (
	log "github.com/Sirupsen/logrus"
)

type User struct {
	connections map[string]*Connection
	chanSend    chan interface{}
	service     *Service

	activeConn *Connection // active conn
	sendUfrag  string
	sendPasswd string
	recvUfrag  string
	recvPasswd string
	offer      string
	answer     string

	utime uint32 // update time
	ctime uint32 // create time
}

func NewUser() *User {
	u := &User{utime: NowMs(), ctime: NowMs()}
	u.connections = make(map[string]*Connection)
	u.chanSend = make(chan interface{}, 100)
	return u
}

func (u *User) getKey() string {
	return u.recvUfrag + ":" + u.sendUfrag
}

func (u *User) setOfferAnswer(offer, answer string) bool {
	var desc1 MediaDesc
	if desc1.Parse([]byte(offer)) {
		// parsed from offer
		u.recvUfrag = desc1.GetUfrag()
		u.recvPasswd = desc1.GetPasswd()
		u.offer = offer
		log.Println("[user] recv ice from offer: ", u.recvUfrag, u.recvPasswd)
	} else {
		log.Println("[user] invalid offer")
		return false
	}

	var desc2 MediaDesc
	if desc2.Parse([]byte(answer)) {
		// parsed from answer
		u.sendUfrag = desc2.GetUfrag()
		u.sendPasswd = desc2.GetPasswd()
		u.answer = answer
		log.Println("[user] send ice from answer: ", u.sendUfrag, u.sendPasswd)
	} else {
		log.Println("[user] invalid answer")
		return false
	}

	u.startService()

	return true
}

func (u *User) getSendIce() (string, string) {
	// parsed from answer
	return u.sendUfrag, u.sendPasswd
}

func (u *User) getRecvIce() (string, string) {
	// parsed from offer
	return u.recvUfrag, u.recvPasswd
}

func (u *User) getOffer() string {
	return u.offer
}

func (u *User) getAnswer() string {
	return u.answer
}

func (u *User) addConnection(conn *Connection) {
	if conn != nil && conn.getAddr() != nil {
		u.connections[NetAddrString(conn.getAddr())] = conn
	} else {
		log.Println("[user] no conn or addr")
	}
}

func (u *User) delConnection(conn *Connection) {
	if conn != nil {
		delete(u.connections, NetAddrString(conn.getAddr()))
	}
}

func (u *User) sendToInner(conn *Connection, data []byte) {
	u.activeConn = conn
	u.chanSend <- data
}

func (u *User) sendToOuter(data []byte) {
	if u.activeConn == nil {
		for k, v := range u.connections {
			if v.isReady() {
				u.activeConn = v
				log.Println("[user] choose active conn, id=", k)
				break
			}
		}
	}
	if u.activeConn != nil {
		u.activeConn.sendData(data)
	} else {
		log.Println("[user] no active connection")
	}
}

func (u *User) isTimeout() bool {
	if len(u.connections) == 0 {
		return true
	}
	return false
}

func (u *User) dispose() {
	log.Println("[user] dispose, connection size=", len(u.connections))
	if u.service != nil {
		u.service.dispose()
	}
	if len(u.connections) > 0 {
		u.connections = make(map[string]*Connection)
	}
}

func (u *User) startService() {
	if u.service != nil {
		return
	}

	log.Println("[user] start service")
	sufrag, spwd := u.getRecvIce()
	rufrag, rpwd := u.getSendIce()
	remoteSdp := genServiceSdp("application", rufrag, rpwd, nil)

	log.Println("[user] init service, sendfragpwd=", sufrag, spwd, len(sufrag), len(spwd))
	log.Println("[user] init service, recvfragpwd=", rufrag, rpwd, len(rufrag), len(rpwd))

	u.service = NewService(u, u.chanSend)
	u.service.Init(sufrag, spwd, remoteSdp)
	u.service.Start()
}
