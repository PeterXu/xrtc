package main

import (
	"flag"
	"fmt"
	"os"

	webrtc "github.com/PeterXu/xrtc/src"
	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
)

func init() {
	log.SetLogDefault()
	log.SetLogFlags(log.LogFlags() | log.Lmilliseconds)
}

func main() {
	var config string
	flag.StringVar(&config, "f", webrtc.DefaultConfig(), "Specify config file")
	flag.Usage = func() {
		fmt.Printf("Usage: %s -h|-f, where\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Println()
	}
	flag.Parse()

	rtc := webrtc.NewWebRTC(config)

	util.AppListen(func(s os.Signal) {
		rtc.Close()
	})

	util.AppWait()
}
