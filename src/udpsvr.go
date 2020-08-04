package webrtc

import (
	//"fmt"
	"net"
	"time"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

type UdpServer struct {
	OneServer

	conn    *net.UDPConn
	stat    *NetStat            // total stat
	clients map[string]*NetStat // stat of each client

	chanRecv chan interface{}
	exitTick chan bool
}

func NewUdpServer(hub *MaxHub, cfg *NetConfig) *UdpServer {
	const TAG = "[UDP]"
	if udpAddr, err := net.ResolveUDPAddr("udp", cfg.Net.Addr); err == nil {
		if conn, err := net.ListenUDP("udp", udpAddr); err == nil {
			log.Println(TAG, "listen udp on:", cfg.Net.Addr)
			util.SetSocketReuseAddr(conn)
			svr := &UdpServer{
				conn:     conn,
				stat:     NewNetStat(0, 0),
				clients:  make(map[string]*NetStat),
				chanRecv: make(chan interface{}, 1000),
				exitTick: make(chan bool),
			}
			svr.Init(TAG, hub, cfg)
			go svr.Run()
			return svr
		} else {
			log.Println(TAG, "listen udp error: ", err)
		}
	} else {
		log.Println(TAG, "resolve addr error: ", err)
	}
	return nil
}

func (u *UdpServer) Run() {
	defer u.conn.Close()

	log.Println(u.TAG, "main begin")

	go u.writeLoop()

	inChan := u.GetDataInChan()
	rbuf := make([]byte, 1024*128)
	for {
		if nret, raddr, err := u.conn.ReadFromUDP(rbuf[0:]); err != nil {
			log.Warnln(u.TAG, "read error: ", err, ", remote: ", raddr)
			break
		} else {
			if _, ok := u.clients[raddr.String()]; !ok {
				u.clients[raddr.String()] = NewNetStat(0, nret)
			}
			//log.Println(u.TAG, "recv msg size: ", nret, ", from ", NetAddrString(raddr))
			u.stat.updateRecv(nret)
			data := make([]byte, nret)
			copy(data, rbuf[0:nret])
			inChan <- NewHubMessage(data, raddr, nil, u.chanRecv)
		}
	}

	u.exitTick <- true

	log.Println(u.TAG, "main end")
}

func (u *UdpServer) writeLoop() {
	tickChan := time.NewTicker(time.Second * 10).C

	for {
		select {
		case msg, ok := <-u.chanRecv:
			if !ok {
				log.Println(u.TAG, "close channel")
				return
			}

			if umsg, ok := msg.(*HubMessage); ok {
				if err, nb := u.SendTo(umsg.data, umsg.to); err != nil {
					log.Warnln(u.TAG, "send err:", err, nb)
				} else {
					//log.Println(u.TAG, "send size:", nb)
				}
			} else {
				log.Warnln(u.TAG, "not-send invalid msg")
			}
		case <-tickChan:
			if !u.stat.checkTimeout(5000) {
				log.Print2f(u.TAG, "statistics, client=%d, stat=%s", len(u.clients), u.stat)
			}
		case <-u.exitTick:
			log.Println(u.TAG, "exit writing")
			return
		}
	}
}

func (u *UdpServer) SendTo(data []byte, to net.Addr) (error, int) {
	if nb, err := u.conn.WriteTo(data, to); err != nil {
		return err, -1
	} else {
		u.stat.updateSend(nb)
		return nil, nb
	}
}
