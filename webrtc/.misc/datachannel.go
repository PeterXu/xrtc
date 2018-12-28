package webrtc

import (
	//"encoding/json"
	//"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

const (
	roleClient = 0
	roleServer = 1

	stateClosed     = 0
	stateConnecting = 1
	stateConnected  = 2
)

type Peer struct {
	ctx        *DtlsContext
	dtls       *DtlsTransport
	sctp       *SctpTransport
	role       int
	remotePort int
	state      int
}

func NewPeer() (*Peer, error) {
	ctx, err := NewContext("gortcdc", 365)
	if err != nil {
		return nil, err
	}
	dtls, err := ctx.NewTransport()
	if err != nil {
		ctx.Destroy()
		return nil, err
	}
	rand.Seed(time.Now().UnixNano())
	sctp, err := NewTransport(rand.Intn(50001) + 10000)
	if err != nil {
		ctx.Destroy()
		dtls.Destroy()
		return nil, err
	}
	p := &Peer{
		ctx:   ctx,
		dtls:  dtls,
		sctp:  sctp,
		role:  roleServer,
		state: stateClosed,
	}
	return p, nil
}

func (p *Peer) Destroy() {
	p.dtls.Destroy()
	p.ctx.Destroy()
	p.sctp.Destroy()
}

type Signaller interface {
	Send(data []byte) error
	ReceiveFrom() <-chan []byte
}

func (p *Peer) Run(s Signaller) error {
	recvChan := s.ReceiveFrom()

	if p.role == roleClient {
		log.Debug("DTLS connecting")
		p.dtls.SetConnectState()
	} else {
		log.Debug("DTLS accepting")
		p.dtls.SetAcceptState()
	}

	// feed data to dtls
	go func() {
		var buf [1 << 16]byte
		for {
			data := <-recvChan
			log.Debug(len(data), " bytes of DTLS data received")
			p.dtls.Feed(data)

			n, _ := p.dtls.Read(buf[:])
			if n > 0 {
				log.Debug(n, " bytes of SCTP data received")
				p.sctp.Feed(buf[:n])
			}
		}
	}()

	// check dtls data
	exitTick := make(chan bool)
	go func() {
		var buf [1 << 16]byte
		tick := time.Tick(4 * time.Millisecond)
		for {
			select {
			case <-tick:
				n, _ := p.dtls.Spew(buf[:])
				if n > 0 {
					log.Debug(n, " bytes of DTLS data ready")
				}
				continue
			case <-exitTick:
				close(exitTick)
				// flush data
				n, _ := p.dtls.Spew(buf[:])
				if n > 0 {
					log.Debug(n, " bytes of DTLS data ready")
				}
				break
			}
			break
		}
	}()

	if err := p.dtls.Handshake(); err != nil {
		return err
	}
	exitTick <- true
	log.Debug("DTLS handshake done")

	// check sctp data
	go func() {
		var buf [1 << 16]byte
		for {
			data := <-p.sctp.BufferChannel
			log.Debug(len(data), " bytes of SCTP data ready")
			p.dtls.Write(data)

			n, _ := p.dtls.Spew(buf[:])
			if n > 0 {
				log.Debug(n, " bytes of DTLS data ready")
			}
		}
	}()

	if p.role == roleClient {
		if err := p.sctp.Connect(p.remotePort); err != nil {
			return err
		}
	} else {
		if err := p.sctp.Accept(); err != nil {
			return err
		}
	}
	p.state = stateConnected
	log.Debug("SCTP handshake done")

	for {
		select {
		case d := <-p.sctp.DataChannel:
			log.Debugf("sid: %d, ppid: %d, data: %v", d.Sid, d.Ppid, d.Data)
		}
	}

	return nil
}

var numbers = []rune("0123456789")

func randSession() string {
	s := make([]rune, 16)
	rand.Seed(time.Now().UnixNano())
	for i := range s {
		s[i] = numbers[rand.Intn(10)]
	}
	return string(s)
}

func (p *Peer) ParseOfferSdp(offer string) (int, error) {
	sdps := strings.Split(offer, "\r\n")
	for i := range sdps {
		if strings.HasPrefix(sdps[i], "a=sctp-port:") {
			sctpmap := strings.Split(sdps[i], " ")[0]
			port, err := strconv.Atoi(strings.Split(sctpmap, ":")[1])
			if err != nil {
				return 0, err
			}
			p.remotePort = port
		} else if strings.HasPrefix(sdps[i], "a=setup:active") {
			if p.role == roleClient {
				p.role = roleServer
			}
		} else if strings.HasPrefix(sdps[i], "a=setup:passive") {
			if p.role == roleServer {
				p.role = roleClient
			}
		}
	}

	p.state = stateConnecting

	return 0, nil
}

func (p *Peer) ParseCandidateSdp(cands string) int {
	sdps := strings.Split(cands, "\r\n")
	count := 0
	for i := range sdps {
		if sdps[i] == "" {
			continue
		}
		count++
	}
	return count
}
