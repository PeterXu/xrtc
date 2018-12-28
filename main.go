package main

import (
	"os"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"

	webrtc "github.com/PeterXu/xrtc/src"
)

func init() {
	log.SetLogDefault()
	log.SetLogFlags(log.LogFlags() | log.Lmilliseconds)
}

func main() {
	hub := webrtc.Inst()

	util.AppListen(func(s os.Signal) {
		hub.Close()
	})

	util.AppWait()
}
