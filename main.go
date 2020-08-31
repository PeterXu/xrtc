package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/PeterXu/xrtc/log"
	webrtc "github.com/PeterXu/xrtc/src"
	"github.com/PeterXu/xrtc/util"
)

func init() {
	log.SetLogDefault()
	log.SetLogFlags(log.LogFlags() | log.Lmilliseconds)
	util.SetLogObject(log.GetObject())
}

func main() {
	var config string
	flag.StringVar(&config, "f", webrtc.DefaultConfig(), "Specify config file")
	flag.Usage = func() {
		fmt.Printf("Usage: %s -h|-f file, where\n", os.Args[0])
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
