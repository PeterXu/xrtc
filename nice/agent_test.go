package nice

import (
	"log"
	"testing"
	"time"
)

func TestNewAgent(t *testing.T) {
	agent, err := NewAgent()
	if err != nil {
		t.Error(err)
	}
	defer agent.Destroy()
}

func TestNewReliableAgent(t *testing.T) {
	agent, err := NewReliableAgent()
	if err != nil {
		t.Error(err)
	}
	defer agent.Destroy()
}

func TestGenerateCandidates(t *testing.T) {
	agent, _ := NewAgent()
	defer agent.Destroy()

	if err := agent.GatherCandidates(); err != nil {
		t.Error(err)
	}

	delay := time.After(2 * time.Second)

	for {
		select {
		case <-delay:
			log.Print("timeout")
			break
		case cand := <-agent.CandidateChannel:
			log.Print(cand)
			continue
		case e := <-agent.EventChannel:
			log.Print(e)
			continue
		}
		break
	}
}

func TestGenerateOffer(t *testing.T) {
	agent, _ := NewAgent()
	defer agent.Destroy()

	log.Print(agent.GenerateSdp())
}

func TestIceNegotiation(t *testing.T) {
	client, _ := NewAgent()
	if err := client.GatherCandidates(); err != nil {
		t.Error(err)
	}

	server, _ := NewAgent()
	if err := server.GatherCandidates(); err != nil {
		t.Error(err)
	}

	// required to get ice ufrag/password
	server.ParseSdp(client.GenerateSdp())
	client.ParseSdp(server.GenerateSdp())

	go client.Run()
	go server.Run()

	clientTimeout := time.After(2 * time.Second)

	go func() {
		for {
			select {
			case cand := <-client.CandidateChannel:
				log.Print("client candidate:", cand)
				// optional if ParseSdp contains candidate
				//server.ParseCandidateSdp(cand)
				continue
			case e := <-client.EventChannel:
				log.Print("client event", e)
				if e.Event == EventNegotiationDone {
					log.Print("client negotiation done")
					client.Send([]byte("hello"))
				}
				continue
			case <-clientTimeout:
				log.Print("client timeout")
				client.Destroy()
				break
			}
			break
		}
	}()

	serverTimeout := time.After(2 * time.Second)

	for {
		select {
		case cand := <-server.CandidateChannel:
			log.Print("server candidate:", cand)
			// optional if ParseSdp contains candidate
			//client.ParseCandidateSdp(cand)
			continue
		case e := <-server.EventChannel:
			log.Print("server event", e)
			if e.Event == EventNegotiationDone {
				log.Print("server negotiation done")
			}
			continue
		case d := <-server.DataChannel:
			log.Print("server received:", string(d))
			continue
		case <-serverTimeout:
			log.Print("server timeout")
			server.Destroy()
			break
		}
		break
	}
}
