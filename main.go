package main

import (
	"os"

	webrtc "github.com/PeterXu/xrtc"
	"github.com/PeterXu/xrtc/exit"
)

func main() {
	TestServer()
}

func TestServer() {
	hub := webrtc.Inst()

	exit.Listen(func(s os.Signal) {
		hub.Exit()
	})

	exit.Wait()
}
