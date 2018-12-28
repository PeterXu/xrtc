package webrtc

import (
	"bytes"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
)

const kDefaultConnectionTimeout = 30 * 1000 // ms

type Connection struct {
	addr     net.Addr
	chanSend chan interface{}
	user     *User

	sendUfrag  string
	sendPasswd string
	recvUfrag  string
	recvPasswd string

	ready                  bool
	stunRequesting         uint32
	hadStunChecking        bool
	hadStunBindingResponse bool
	bindError              int
	leave                  bool

	utime uint32 // update time
	ctime uint32 // create time
}

func NewConnection(addr net.Addr, chanSend chan interface{}) *Connection {
	c := &Connection{addr: addr, chanSend: chanSend, utime: NowMs(), ctime: NowMs()}
	c.ready = false
	c.hadStunChecking = false
	c.hadStunBindingResponse = false
	c.bindError = 0
	c.leave = false
	return c
}

func (c *Connection) setSendIce(ufrag, pwd string) {
	// parsed from answer
	c.sendUfrag = ufrag
	c.sendPasswd = pwd
	log.Println("[conn] send ice: ", ufrag, pwd)
}

func (c *Connection) setRecvIce(ufrag, pwd string) {
	// parsed from offer
	c.recvUfrag = ufrag
	c.recvPasswd = pwd
	log.Println("[conn] recv ice: ", ufrag, pwd)
}

func (c *Connection) setUser(user *User) {
	c.user = user
	if user != nil {
		c.setSendIce(user.getSendIce())
		c.setRecvIce(user.getRecvIce())
	}
}

func (c *Connection) getAddr() net.Addr {
	return c.addr
}

func (c *Connection) dispose() {
	if c.user != nil {
		c.user.delConnection(c)
	}
}

func (c *Connection) isTimeout() bool {
	if NowMs() >= (c.utime + kDefaultConnectionTimeout) {
		return true
	}
	return false
}

func (c *Connection) onRecvData(data []byte) {
	c.utime = NowMs()

	if IsStunPacket(data) {
		var msg IceMessage
		if !msg.Read(data) {
			log.Warnln("[conn] invalid stun message, dtype=", msg.dtype)
			return
		}

		switch msg.dtype {
		case STUN_BINDING_REQUEST:
			c.onRecvStunBindingRequest(msg.transId)
		case STUN_BINDING_RESPONSE:
			if c.hadStunBindingResponse {
				log.Println("[conn] had stun binding response")
				return
			}
			log.Println("[conn] recv stun binding response")
			// init and enable srtp
			c.hadStunBindingResponse = true
			c.ready = true
		}
	} else {
		// dtls handshake
		// rtp/rtcp data to inner
		//log.Println("[conn] recv dtls/rtp/rtcp, len=", len(data))
		c.ready = true
		c.user.sendToInner(c, data)
	}
}

func (c *Connection) onRecvStunBindingRequest(transId string) {
	if c.leave {
		log.Println("[conn] had left!")
		return
	}

	//log.Println("[conn] send stun binding response")
	resp := &StunMessage{dtype: STUN_BINDING_RESPONSE, transId: transId}

	xorAttr := &StunXorAddressAttribute{}
	xorAttr.SetType(STUN_ATTR_XOR_MAPPED_ADDRESS)
	xorAttr.addr.SetAddr(c.addr)
	resp.AddAttribute(xorAttr)
	resp.AddMessageIntegrity(c.sendPasswd)
	resp.AddFingerprint()

	var buf bytes.Buffer
	if !resp.Write(&buf) {
		log.Println("[conn] fail to gen stun response")
		return
	}

	//log.Println("[conn] stun response len=", len(buf.Bytes()))
	c.sendData(buf.Bytes())
	c.checkStunBindingRequest()
}

func (c *Connection) isReady() bool {
	return c.ready
}

func (c *Connection) sendData(data []byte) bool {
	c.chanSend <- NewHubMessage(data, nil, c.addr, nil)
	return true
}

func (c *Connection) sendStunBindingRequest() bool {
	if c.hadStunBindingResponse || c.bindError != 0 {
		return false
	}

	//log.Println("[conn] send stun binding request")
	req := &StunMessage{
		dtype:   STUN_BINDING_REQUEST,
		transId: RandomString(kStunTransactionIdLength),
	}

	sendKey := c.recvUfrag + ":" + c.sendUfrag
	usernameAttr := NewStunByteStringAttribute(STUN_ATTR_USERNAME, []byte(sendKey))
	req.AddAttribute(usernameAttr)
	req.AddMessageIntegrity(c.recvPasswd)
	req.AddFingerprint()

	var buf bytes.Buffer
	if req.Write(&buf) {
		log.Println("[conn] send stun binding request, len=", buf.Len())
		c.sendData(buf.Bytes())
	} else {
		log.Println("[conn] fail to get stun request bufffer")
	}
	return true
}

func (c *Connection) checkStunBindingRequest() {
	if !c.sendStunBindingRequest() {
		return
	}

	if c.hadStunChecking {
		return
	}

	c.hadStunChecking = true

	go func() {
		c.stunRequesting = 500
		for {
			select {
			case <-time.After(time.Millisecond * time.Duration(c.stunRequesting)):
				if !c.sendStunBindingRequest() {
					log.Println("[conn] quit stun request interval")
					c.hadStunChecking = false
					return
				}

				if delta := NowMs() - c.utime; delta >= (15 * 1000) {
					log.Println("[conn] no response from client and quit")
					return
				} else if delta > (5 * 1000) {
					log.Println("[conn] adjust stun request interval")
					c.stunRequesting = delta / 2
				} else if delta < 500 {
					c.stunRequesting = 500
				}
			}
		}
	}()
}
