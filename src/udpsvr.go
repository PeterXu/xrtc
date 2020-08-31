package webrtc

import (
	//"fmt"
	"net"
	"time"

	"github.com/PeterXu/xrtc/log"
	"github.com/PeterXu/xrtc/util"
)

type UdpServer struct {
	*OneServiceHandler
	TAG     string
	owner   ServiceOwner
	addr    string
	conn    *net.UDPConn
	clients map[string]*NetStat // stat of each client
}

func NewUdpServer(owner ServiceOwner, addr string) *UdpServer {
	const TAG = "[UDP]"
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Println(TAG, "resolve addr error: ", err)
		return nil
	}

	log.Println(TAG, "listen udp on:", addr)
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Println(TAG, "listen udp error: ", err)
		return nil
	}

	util.SetSocketReuseAddr(conn)
	svr := &UdpServer{
		OneServiceHandler: NewOneServiceHandler(),
		TAG:               TAG,
		owner:             owner,
		addr:              addr,
		conn:              conn,
		clients:           make(map[string]*NetStat),
	}
	go svr.Run()
	return svr
}

func (u *UdpServer) Close() {
}

func (u *UdpServer) Run() {
	log.Println(u.TAG, "main begin")

	defer u.conn.Close()

	go u.writeLoop()

	rbuf := make([]byte, 1024*128)
	for {
		if nret, raddr, err := u.conn.ReadFromUDP(rbuf[0:]); err == nil {
			if _, ok := u.clients[raddr.String()]; !ok {
				u.clients[raddr.String()] = NewNetStat(0, nret)
			}
			//log.Println(u.TAG, "recv msg size: ", nret, ", from ", NetAddrString(raddr))
			u.netStat.UpdateRecv(nret)
			u.owner.OnRecvData(rbuf[0:nret], raddr, u)
		} else {
			log.Warnln(u.TAG, "read error: ", err, ", remote: ", raddr)
			break
		}
	}

	u.exitTick <- true

	log.Println(u.TAG, "main end")
}

func (u *UdpServer) writeLoop() {
	tickChan := time.NewTicker(time.Second * 10).C

exitLoop:
	for {
		select {
		case msg, ok := <-u.chanFeed:
			if !ok {
				log.Println(u.TAG, "writeLoop exit: channel closed")
				break exitLoop
			}

			if umsg, ok := msg.(*ObjMessage); ok {
				if err, nb := u.sendToInternal(umsg.data, umsg.to); err != nil {
					log.Warnln(u.TAG, "writeLoop send err:", err, nb)
				} else {
					//log.Println(u.TAG, "send size:", nb)
				}
			} else {
				log.Warnln(u.TAG, "writeLoop not-send invalid msg")
			}
		case <-tickChan:
			if !u.netStat.CheckTimeout(5000) {
				log.Print2f(u.TAG, "writeLoop statistics, client=%d, stat=%s", len(u.clients), u.netStat)
			}
		case <-u.exitTick:
			log.Println(u.TAG, "writeLoop exit writing")
			break exitLoop
		}
	}
	close(u.exitTick)
}

func (u *UdpServer) sendToInternal(data []byte, to net.Addr) (error, int) {
	if nb, err := u.conn.WriteTo(data, to); err != nil {
		return err, -1
	} else {
		u.netStat.UpdateSend(nb)
		return nil, nb
	}
}
