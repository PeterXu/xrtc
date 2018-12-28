package webrtc

import (
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

type Service struct {
	agent *Agent
	user  *User

	ready    bool
	chanRecv chan interface{}

	sendCount int
	recvCount int

	// exit chan
	exitTick chan bool

	utime uint32 // update time
	ctime uint32 // create time
}

func NewService(user *User, chanRecv chan interface{}) *Service {
	s := &Service{
		ready:    false,
		user:     user,
		chanRecv: chanRecv,
		exitTick: make(chan bool),
		utime:    NowMs(),
		ctime:    NowMs(),
	}
	return s
}

func (s *Service) Init(ufrag, pwd, remote string) bool {
	iceDebugEnable(true)

	s.agent, _ = NewAgent()
	s.agent.SetMinMaxPort(40000, 50000)
	s.agent.SetLocalCredentials(ufrag, pwd)
	if err := s.agent.GatherCandidates(); err != nil {
		log.Println("[service] gather error:", err)
		return false
	}

	//local := s.agent.GenerateSdp()
	//log.Println("[service] local sdp:", local)

	//log.Println("[service] remote sdp:", remote)
	// required to get ice ufrag/password
	if _, err := s.agent.ParseSdp(remote); err != nil {
		log.Println("[service] ParseSdp, err=", err)
		return false
	}

	// optional if ParseSdp contains condidates
	//s.agent.ParseCandidateSdp(cand)
	return true
}

func (s *Service) Start() {
	if s.agent != nil {
		go s.Run()
	} else {
		log.Println("[service] no valid agent")
	}
}

func (s *Service) dispose() {
	log.Println("[service] dispose begin")
	if s.agent != nil {
		s.agent.Destroy()
		s.agent = nil
	}
	s.exitTick <- true
	log.Println("[service] dispose end")
}

func (s *Service) ChanRecv() chan interface{} {
	if s.ready {
		return s.chanRecv
	}
	return nil
}

func (s *Service) Run() {
	log.Println("[service] begin")
	go s.agent.Run()

	agentKey := s.user.getKey()
	_ = agentKey

	tickChan := time.NewTicker(time.Second * 5).C

	for {
		select {
		case msg, ok := <-s.ChanRecv():
			if !ok {
				log.Println("[service] close chanRecv")
				return
			}
			if data, ok := msg.([]byte); ok {
				//log.Println("[service] forward data to inner, size=", len(data))
				s.sendCount += len(data)
				s.agent.Send(data)
			}
			continue
		case cand := <-s.agent.CandidateChannel:
			//log.Println("[service] agent candidate:", cand)
			// send to server
			_ = cand
			continue
		case e := <-s.agent.EventChannel:
			if e.Event == EventNegotiationDone {
				log.Println("[service] agent negotiation done")
				// dtls handshake/sctp
				//s.agent.Send([]byte("hello"))
			} else if e.Event == EventStateChanged {
				switch e.State {
				case EventStateNiceDisconnected:
					s.ready = false
					log.Println("[service] agent ice disconnected")
				case EventStateNiceConnected:
					s.ready = true
					log.Println("[service] agent ice connected")
				case EventStateNiceReady:
					s.ready = true
					log.Println("[service] agent ice ready")
				default:
					s.ready = false
					log.Println("[service] agent ice state:", e.State)
				}
			} else {
				log.Println("[service] unknown agent event:", e)
			}
			continue
		case d := <-s.agent.DataChannel:
			//log.Println("[service] agent received:", len(d))
			// dtls handshake/sctp
			s.recvCount += len(d)
			s.user.sendToOuter(d)
			continue
		case <-tickChan:
			//log.Printf("[service] agent[%s] statistics, sendCount=%d, recvCount=%d\n", agentKey, s.sendCount, s.recvCount)
			continue
		case <-s.exitTick:
			close(s.exitTick)
			break
		}
		break
	}
	log.Println("[service] end")
}

func genServiceSdp(media, ufrag, pwd string, candidates []string) string {
	const kDefaultUdpCandidate = "a=candidate:3159811271 1 udp 2113937151 119.254.195.20 5000 typ host"
	const kDefaultTcpCandidate = "a=candidate:4074051639 1 tcp 1518280447 119.254.195.20 443 typ host tcptype passive"

	var lines []string
	lines = append(lines, "m="+media)
	lines = append(lines, "c=IN IP4 0.0.0.0")
	lines = append(lines, "a=ice-ufrag:"+ufrag)
	lines = append(lines, "a=ice-pwd:"+pwd)
	lines = append(lines, kDefaultUdpCandidate)
	//lines = append(lines, kDefaultTcpCandidate)
	//lines = append(lines, candidates...)
	return strings.Join(lines, "\n")
}
