package main

import (
	"log"
	"os"
	"time"

	webrtc "github.com/PeterXu/xrtc/src"
	"github.com/PeterXu/xrtc/util"
)

func main() {
	//TestServer()
	//TestIce()
	TestYaml()
}

func TestServer() {
	hub := webrtc.Inst()

	util.AppListen(func(s os.Signal) {
		hub.Exit()
	})

	util.AppWait()
}

const kSdpSample = `m=application 0 ICE/SDP
c=IN IP4 0.0.0.0
a=ice-ufrag:PAmP
a=ice-pwd:MgHkN8dlHmhKSPPXMEhg7O
a=candidate:3159811271 1 udp 2113937151 119.254.195.20 5000 typ host
a=candidate:4074051639 1 tcp 1518280447 119.254.195.20 443 typ host tcptype passive
`

func TestIce() {
	client, _ := webrtc.NewAgent()
	if err := client.GatherCandidates(); err != nil {
		log.Println("gather error:", err)
		return
	}

	//sdp := client.GenerateSdp()
	//log.Println("sdp:", sdp)

	log.Println("head:", kSdpSample)
	client.ParseSdp(kSdpSample)

	go client.Run()

	clientTimeout := time.After(30 * time.Second)

	go func() {
		for {
			select {
			case cand := <-client.CandidateChannel:
				log.Print("client candidate:", cand)
				// send to server
				continue
			case e := <-client.EventChannel:
				log.Print("client event", e)
				if e.Event == webrtc.EventNegotiationDone {
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

	exit.Listen(func(s os.Signal) {
	})
	exit.Wait()
}

func TestYaml() {
	var fname string = "./routes.yml"
	log.Println("[yaml] load file,", fname)
	config := webrtc.NewConfig()
	if !config.Load(fname) {
		log.Fatalf("[yaml] read config failed")
		return
	}
	log.Println("[yaml] load success")

	hub := webrtc.Inst()
	log.Println("[yaml] candidates:", hub.Candidates())
}
