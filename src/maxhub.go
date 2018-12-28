package webrtc

import (
	"net"
	"strings"
	"time"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

type OneServer interface {
	Run()
	Close()
	Params() *NetParams
}

type HubMessage struct {
	data []byte
	from net.Addr
	to   net.Addr
	misc interface{}
}

func NewHubMessage(data []byte, from net.Addr, to net.Addr, misc interface{}) *HubMessage {
	return &HubMessage{data, from, to, misc}
}

type MaxHub struct {
	TAG string

	connections map[string]*Connection
	clients     map[string]*User
	servers     []OneServer

	// cache control
	cache *Cache

	// data from outer client(over udpsvr/tcpsvr)
	chanRecvFromOuter chan interface{}

	// admin chan
	chanAdmin chan interface{}

	// exit chan
	exitTick chan bool
}

func NewMaxHub() *MaxHub {
	hub := &MaxHub{
		TAG:               "[MAXHUB]",
		connections:       make(map[string]*Connection),
		clients:           make(map[string]*User),
		cache:             NewCache(),
		chanRecvFromOuter: make(chan interface{}, 1000), // unblocking mode, data from udpsvr
		chanAdmin:         make(chan interface{}, 10),   // data from admin/control
		exitTick:          make(chan bool),
	}
	go hub.Run()
	return hub
}

func (h *MaxHub) OnAdminData(msg *HubMessage) {
}

func (h *MaxHub) findConnection(addr net.Addr) *Connection {
	var key string = util.NetAddrString(addr)
	if u, ok := h.connections[key]; ok {
		return u
	}
	return nil
}

func (h *MaxHub) handleStunBindingRequest(data []byte, addr net.Addr, misc interface{}) {
	var msg util.IceMessage
	if !msg.Read(data) {
		log.Warnln(h.TAG, "invalid stun message")
		return
	}

	log.Println(h.TAG, "proc stun message")
	switch msg.Dtype {
	case util.STUN_BINDING_REQUEST:
		attr := msg.GetAttribute(util.STUN_ATTR_USERNAME)
		if attr == nil {
			log.Warnln(h.TAG, "no stun attr of username")
			return
		}

		// format: "answer_ufrag:offer_ufrag"
		stunName := string(attr.(*util.StunByteStringAttribute).Data)
		items := strings.Split(stunName, ":")
		if len(items) != 2 {
			log.Warnln(h.TAG, "invalid stun name:", stunName)
			return
		}

		log.Println(h.TAG, "stun name:", items)

		user, ok := h.clients[stunName]
		if !ok {
			var request *RegisterRequest
			if item := h.cache.Get(stunName); item != nil {
				if info, ok := item.data.(*RegisterRequest); ok {
					request = info
				}
			}
			if request == nil {
				log.Warnln(h.TAG, "invalid ice for user")
				return
			}
			iceTcp := false
			iceDirect := true
			user = NewUser(iceTcp, iceDirect)
			if !user.setIceInfo(&request.OfferIce, &request.AnswerIce, request.Candidates) {
				log.Warnln(h.TAG, "invalid ice for user")
				return
			}
			h.clients[stunName] = user
		} else {
			log.Warnln(h.TAG, "another connection for user-stun=", stunName)
		}

		if chanSend, ok := misc.(chan interface{}); ok {
			// new conn
			conn := NewConnection(addr, chanSend)
			conn.setUser(user)
			// add conn into user
			user.addConnection(conn)
			h.connections[util.NetAddrString(addr)] = conn
			conn.onRecvData(data)
		} else {
			log.Warnln(h.TAG, "no chanSend for this connection")
		}
	default:
		log.Warnln(h.TAG, "invalid stun type =", msg.Dtype)
	}
}

func (h *MaxHub) clearConnections() {
	var connKeys []string
	for k, v := range h.connections {
		if v.isTimeout() {
			v.dispose()
			connKeys = append(connKeys, k)
		}
	}

	if len(connKeys) > 0 {
		log.Println(h.TAG, "clear connections, size=", len(connKeys))
		for index := range connKeys {
			delete(h.connections, connKeys[index])
		}
	}
}

func (h *MaxHub) clearUsers() {
	var userKeys []string
	for k, v := range h.clients {
		if v.isTimeout() {
			v.dispose()
			userKeys = append(userKeys, k)
		}
	}

	if len(userKeys) > 0 {
		log.Println(h.TAG, "clear users, size=", len(userKeys))
		for index := range userKeys {
			delete(h.clients, userKeys[index])
		}
	}
}

func (h *MaxHub) OnRecvFromOuter(msg *HubMessage) {
	// 1. stun request/response
	// 2. dtls handshake(key)
	// 3. sctp create/srtp init
	//log.Println(h.TAG, "data from outer")
	if conn := h.findConnection(msg.from); conn != nil {
		conn.onRecvData(msg.data)
	} else {
		if util.IsStunPacket(msg.data) {
			h.handleStunBindingRequest(msg.data, msg.from, msg.misc)
		} else {
			log.Warnln(h.TAG, "invalid data from outer")
		}
	}
}

// request from outer (browser clients)
func (h *MaxHub) ChanRecvFromOuter() chan interface{} {
	return h.chanRecvFromOuter
}

func (h *MaxHub) AddServer(server OneServer) {
	if server != nil {
		h.servers = append(h.servers, server)
	}
}

func (h *MaxHub) Cache() *Cache {
	return h.cache
}

func (h *MaxHub) Candidates() []string {
	var candidates []string
	for _, svr := range h.servers {
		candidates = append(candidates, svr.Params().Candidates...)
	}
	return candidates
}

func (h *MaxHub) Close() {
	for _, svr := range h.servers {
		svr.Close()
	}
	h.exitTick <- true
	h.cache.Close()
}

func (h *MaxHub) Run() {
	log.Println(h.TAG, "Run begin")

	errCh := make(chan error)

	go h.loopForOuter(errCh)

	quit := false
	for !quit {
		select {
		case msg, ok := <-h.chanAdmin:
			if ok {
				h.OnAdminData(msg.(*HubMessage))
			}
		case <-h.exitTick:
			quit = true
			errCh <- nil
			log.Println(h.TAG, "Run exit...")
		}
	}
	log.Println(h.TAG, "Run end")
}

func (h *MaxHub) loopForOuter(errCh chan error) {
	tickChan := time.NewTicker(time.Second * 30).C

	quit := false
	for !quit {
		select {
		case msg, ok := <-h.chanRecvFromOuter:
			if ok {
				h.OnRecvFromOuter(msg.(*HubMessage))
			}
		case <-tickChan:
			h.clearConnections()
			h.clearUsers()
		case <-errCh:
			quit = true
		}
	}
}
