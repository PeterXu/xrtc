package webrtc

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/PeterXu/xrtc/nice"
	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

type IceAgent struct {
	*ObjTime
	*ObjStatus
	*NetHandler

	TAG   string
	agent *nice.Agent
	user  *User

	// when iceDirect == true
	iceInChan  chan []byte
	iceOutChan chan []byte
	iceCands   []util.Candidate
	remoteAddr net.Addr
}

func NewIceAgent(user *User, chanRecv chan interface{}) *IceAgent {
	return &IceAgent{
		ObjTime:    NewObjTime(),
		ObjStatus:  NewObjStatus(),
		NetHandler: NewNetHandler(),
		TAG:        "[ICE-AGENT]",
		user:       user,
	}
}

func (s *IceAgent) Init(ufrag, pwd, remote string) bool {
	log.Println(s.TAG, "Init begin")
	if s.user.isIceDirect() {
		var desc util.MediaDesc
		if desc.Parse([]byte(remote)) {
			s.iceCands, _ = util.ParseCandidates(desc.GetCandidates())
			log.Println(s.TAG, "Init candidates", s.iceCands)
			// connect server with cands
			s.iceInChan = make(chan []byte, 100)
			s.iceOutChan = make(chan []byte, 100)
		} else {
			log.Warnln(s.TAG, "fail to parse sdp:", remote)
			return false
		}
		return true
	}

	//iceDebugEnable(true)
	s.agent, _ = nice.NewAgent()
	s.agent.SetMinMaxPort(40000, 50000)
	s.agent.SetLocalCredentials(ufrag, pwd)
	log.Println(s.TAG, "Init gathering..")
	if err := s.agent.GatherCandidates(); err != nil {
		log.Warnln(s.TAG, "gather error:", err)
		return false
	}

	//local := s.agent.GenerateSdp()
	//log.Println("[service] local sdp:", local)

	//log.Println("[service] remote sdp:", remote)
	// required to get ice ufrag/password
	if _, err := s.agent.ParseSdp(remote); err != nil {
		log.Warnln(s.TAG, "ParseSdp, err=", err)
		return false
	}

	// optional if ParseSdp contains condidates
	//s.agent.ParseCandidateSdp(cand)

	log.Println(s.TAG, "Init ok")
	return true
}

func (s *IceAgent) onRecvData(data []byte) {
	s.netStat.UpdateRecv(len(data))
	s.user.onServerData(data)
}

// sendData sends stun/dtls/srtp/srtcp packets to inner(webrtc server)
func (s *IceAgent) sendData(data []byte) {
	if !s.IsReady() {
		log.Warnln(s.TAG, "inner not ready")
		return
	}

	s.netStat.UpdateSend(len(data))
	if s.agent != nil {
		s.agent.Send(data)
	} else {
		if !s.user.isIceDirect() {
			log.Warnln(s.TAG, "not agent/iceDirect")
			return
		}
		s.iceOutChan <- data
	}
}

func (s *IceAgent) eventChannel() chan *nice.GoEvent {
	if s.agent != nil {
		return s.agent.EventChannel
	} else {
		return nil
	}
}

func (s *IceAgent) candidateChannel() chan string {
	if s.agent != nil {
		return s.agent.CandidateChannel
	} else {
		return nil
	}
}

func (s *IceAgent) dataChannel() chan []byte {
	if s.agent != nil {
		return s.agent.DataChannel
	} else {
		if !s.IsReady() {
			return nil
		}
		return s.iceInChan
	}
}

func (s *IceAgent) Start() bool {
	if s.agent != nil {
		go s.agent.Run()
	} else {
		retCh := make(chan error)
		go s.iceLoop(retCh)
		if err := <-retCh; err != nil {
			log.Warnln(s.TAG, "Start failed:", err)
			return false
		}
	}

	go s.Run()

	return true
}

func (s *IceAgent) dispose() {
	log.Println(s.TAG, "dispose begin")
	if s.agent != nil {
		s.agent.Destroy()
		s.agent = nil
	}
	s.exitTick <- true
	log.Println(s.TAG, "dispose end")
}

func (s *IceAgent) ChanRecv() chan interface{} {
	if s.IsReady() {
		return s.chanFeed
	}
	return nil
}

// iceLoop works when iceDirect is on
func (s *IceAgent) iceLoop(retCh chan error) {
	var tcpCands []util.Candidate
	var udpCands []util.Candidate
	for _, cand := range s.iceCands {
		if cand.CandType != "typ host" {
			continue
		}
		if cand.Transport == "tcp" {
			if cand.NetType == "tcptype passive" {
				tcpCands = append(tcpCands, cand)
			}
		} else {
			udpCands = append(udpCands, cand)
		}
	}

	var cands []util.Candidate
	if s.user.isIceTcp() {
		cands = append(cands, tcpCands...)
		cands = append(cands, udpCands...)
	} else {
		cands = append(cands, udpCands...)
		cands = append(cands, tcpCands...)
	}

	var isTcp bool
	var conn net.Conn
	for _, cand := range cands {
		isTcp = (cand.Transport == "tcp")

		var err error
		addr := fmt.Sprintf("%s:%s", cand.RelAddr, cand.RelPort)
		if conn, err = net.Dial(cand.Transport, addr); err != nil {
			log.Warnln(s.TAG, "connect fail", addr, err)
			continue
		}

		log.Println(s.TAG, "connect ok to", cand.Transport, addr)
		s.SetReady()
		break
	}

	if !s.IsReady() {
		log.Warnln(s.TAG, "fail conn for ice")
		retCh <- errors.New("ice to server failed")
		return
	} else {
		log.Println(s.TAG, "success conn for ice, isTcp:", isTcp)
		s.remoteAddr = conn.RemoteAddr()
		retCh <- nil
	}

	defer conn.Close()

	errCh := make(chan error)

	// read loop
	go func(errCh chan error) {
		rbuf := make([]byte, 1024*128)
		for {
			var nret int
			var err error
			if isTcp {
				nret, err = util.ReadIceTcpPacket(conn, rbuf[0:])
			} else {
				nret, err = conn.Read(rbuf)
			}
			//log.Println(s.TAG, "read loop, isTcp:", isTcp, nret)
			if err == nil {
				if nret > 0 {
					s.iceInChan <- util.Clone(rbuf[0:nret])
				} else {
					log.Warnln(s.TAG, "read data nothing")
				}
			} else {
				errCh <- err
				break
			}
		}
	}(errCh)

	// write loop
exitLoop:
	for {
		select {
		case data := <-s.iceOutChan:
			var nb int
			var err error
			if isTcp {
				nb, err = util.WriteIceTcpPacket(conn, data)
			} else {
				nb, err = conn.Write(data)
			}
			if err != nil {
				log.Warnln(s.TAG, "write data err:", err)
			} else {
				//log.Println(s.TAG, "write data nb:", nb, len(data), isTcp)
				_ = nb
			}
		case err := <-errCh:
			log.Warnln(s.TAG, "read data err:", err)
			break exitLoop
		}
	}

	s.exitTick <- true
}

func (s *IceAgent) Run() {
	log.Println(s.TAG, "Run begin")

	agentKey := s.user.getIceKey()
	_ = agentKey

	tickChan := time.NewTicker(time.Second * 10).C

exitLoop:
	for {
		select {
		case msg, ok := <-s.ChanRecv():
			if ok {
				if data, isok := msg.([]byte); isok {
					//log.Println(s.TAG, "forward data to inner, size=", len(data))
					s.sendData(data)
				}
			} else {
				log.Println(s.TAG, "close chanRecv")
				break exitLoop
			}
		case cand := <-s.candidateChannel():
			//log.Println(s.TAG, "agent candidate:", cand)
			// send to server
			_ = cand
		case e := <-s.eventChannel():
			if e.Event == nice.EventNegotiationDone {
				log.Println(s.TAG, "agent negotiation done")
				// dtls handshake/sctp
				//s.agent.Send([]byte("hello"))
			} else if e.Event == nice.EventStateChanged {
				switch e.State {
				case nice.EventStateNiceDisconnected:
					s.SetClose()
					log.Println(s.TAG, "agent ice disconnected")
					break exitLoop
				case nice.EventStateNiceConnected:
					s.SetReady()
					log.Println(s.TAG, "agent ice connected")
				case nice.EventStateNiceReady:
					s.SetReady()
					log.Println(s.TAG, "agent ice ready")
				default:
					s.SetClose()
					log.Println(s.TAG, "agent ice state:", e.State)
				}
			} else {
				log.Warnln(s.TAG, "unknown agent event:", e)
			}
		case d := <-s.dataChannel():
			// dtls handshake/sctp
			//log.Println(s.TAG, "agent received:", len(d))
			s.onRecvData(d)
		case <-tickChan:
			if !s.netStat.CheckTimeout(5000) {
				log.Print2f(s.TAG, "agent[%s] stat - %s\n", agentKey, s.netStat)
			}
		case <-s.exitTick:
			break exitLoop
		}
	}

	s.user.onAgentClose()
	log.Println(s.TAG, "Run end")
}

func genIceAgentSdp(ufrag, pwd string, candidates []string) string {
	var lines []string
	lines = append(lines, "m=application")
	lines = append(lines, "c=IN IP4 0.0.0.0")
	lines = append(lines, "a=ice-ufrag:"+ufrag)
	lines = append(lines, "a=ice-pwd:"+pwd)
	lines = append(lines, candidates...)
	return strings.Join(lines, "\n")
}
