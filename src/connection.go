package webrtc

import (
	"bytes"
	"net"
	"time"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

const kDefaultConnectionTimeout = 30 * 1000 // ms

type Connection struct {
	TAG      string
	addr     net.Addr
	chanSend chan interface{}
	user     *User

	ready                  bool
	stunRequesting         uint64
	hadStunChecking        bool
	hadStunBindingResponse bool
	leave                  bool
	objtime                *ObjTime
}

func NewConnection(addr net.Addr, chanSend chan interface{}) *Connection {
	return &Connection{
		TAG:                    "[CONN]",
		addr:                   addr,
		chanSend:               chanSend,
		ready:                  false,
		hadStunChecking:        false,
		hadStunBindingResponse: false,
		leave:                  false,
		objtime:                NewObjTime(),
	}
}

func (c *Connection) setUser(user *User) {
	c.user = user
}

func (c *Connection) getAddr() net.Addr {
	return c.addr
}

func (c *Connection) dispose() {
	c.leave = true
	if c.user != nil {
		c.user.delConnection(c)
	}
}

func (c *Connection) isTimeout() bool {
	return c.objtime.checkTimeout(kDefaultConnectionTimeout)
}

func (c *Connection) onRecvData(data []byte) {
	c.objtime.update()

	if !c.user.isIceDirect() && util.IsStunPacket(data) {
		log.Println(c.TAG, "recv stun, len=", len(data))
		var msg util.IceMessage
		if !msg.Read(data) {
			log.Warnln(c.TAG, "invalid stun message, dtype=", msg.Dtype)
			return
		}

		switch msg.Dtype {
		case util.STUN_BINDING_REQUEST:
			c.onRecvStunBindingRequest(msg.TransId)
		case util.STUN_BINDING_RESPONSE:
			if c.hadStunBindingResponse {
				log.Warnln(c.TAG, "had stun binding response")
				return
			}
			log.Println(c.TAG, "recv stun binding response")
			// init and enable srtp
			c.hadStunBindingResponse = true
			c.ready = true
		case util.STUN_BINDING_ERROR_RESPONSE:
			log.Warnln(c.TAG, "error stun message")
		default:
			log.Warnln(c.TAG, "unknown stun message=", msg.Dtype)
		}
	} else {
		// dtls handshake
		// rtp/rtcp data to inner
		//log.Println(c.TAG, "recv dtls/rtp/rtcp, len=", len(data))
		c.ready = true
		c.user.sendToInner(c, data)
	}
}

func (c *Connection) sendData(data []byte) bool {
	c.chanSend <- NewHubMessage(data, nil, c.addr, nil)
	return true
}

func (c *Connection) isReady() bool {
	return c.ready
}

func (c *Connection) onRecvStunBindingRequest(transId string) {
	if c.leave {
		log.Warnln(c.TAG, "had left!")
		return
	}

	log.Println(c.TAG, "recv request and send stun binding response")
	sendIce := c.user.getSendIce()

	var buf bytes.Buffer
	if !util.GenStunMessageResponse(&buf, sendIce.Pwd, transId, c.addr) {
		log.Warnln(c.TAG, "fail to gen stun response")
		return
	}

	log.Println(c.TAG, "stun response len=", len(buf.Bytes()))
	c.sendData(buf.Bytes())
	c.checkStunBindingRequest()
}

func (c *Connection) sendStunBindingRequest() bool {
	if c.hadStunBindingResponse {
		return false
	}

	log.Println(c.TAG, "send stun binding request")
	sendIce := c.user.getSendIce()
	recvIce := c.user.getRecvIce()

	var buf bytes.Buffer
	if util.GenStunMessageRequest(&buf, sendIce.Ufrag, recvIce.Ufrag, recvIce.Pwd) {
		log.Println(c.TAG, "send stun binding request, len=", buf.Len())
		c.sendData(buf.Bytes())
	} else {
		log.Warnln(c.TAG, "fail to get stun request bufffer")
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
					log.Println(c.TAG, "quit stun request interval")
					c.hadStunChecking = false
					return
				}

				if delta := util.NowMs64() - c.objtime.utime; delta >= (15 * 1000) {
					log.Warnln(c.TAG, "(timeout) no response from client and quit")
					return
				} else if delta > (5 * 1000) {
					log.Println(c.TAG, "adjust stun request interval")
					c.stunRequesting = delta / 2
				} else if delta < 500 {
					c.stunRequesting = 500
				}
			}
		}
	}()
}
