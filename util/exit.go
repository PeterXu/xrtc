// Package exit allows to register callbacks which are called on program exit.
//
// Based-on https://github.com/fabiolb/fabio/exit.
package util

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var app_wg sync.WaitGroup

// quit channel is closed to cleanup exit listeners.
var app_quit = make(chan bool)

// Listen registers an exit handler which is called on
// SIGINT/SIGTERM or when Exit/Fatal/Fatalf is called.
// SIGHUP is ignored since that is usually used for
// triggering a reload of configuration which isn't
// supported but shouldn't kill the process either.
func AppListen(fn func(os.Signal)) {
	app_wg.Add(1)
	go func() {
		defer app_wg.Done()
		for {
			sigchan := make(chan os.Signal, 1)
			signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

			var sig os.Signal
			select {
			case sig = <-sigchan:
				switch sig {
				case syscall.SIGHUP:
					Println(uTAG, "Caught SIGHUP. Ignoring")
					continue
				case os.Interrupt:
					Println(uTAG, "Caught SIGINT. Exiting")
				case syscall.SIGTERM:
					Println(uTAG, "Caught SIGTERM. Exiting")
				default:
					// fallthrough in case we forgot to add a switch clause.
					Print2f(uTAG, "Caught signal %v. Exiting", sig)
				}
			case <-app_quit:
			}
			if fn != nil {
				fn(sig)
			}
			return
		}
	}()
}

// Exit terminates the application via os.Exit but
// waits for all exit handlers to complete before
// calling os.Exit.
func AppExit(code int) {
	defer func() { recover() }() // don't panic if close(quit) is called concurrently
	close(app_quit)
	app_wg.Wait()
	os.Exit(code)
}

// Fatal is a replacement for log.Fatal which will trigger
// the closure of all registered exit handlers and waits
// for their completion and then call os.Exit(1).
func AppFatal(v ...interface{}) {
	log.Print(v...)
	AppExit(1)
}

// Fatalf is a replacement for log.Fatalf and behaves like Fatal.
func AppFatalf(format string, v ...interface{}) {
	log.Printf(format, v...)
	AppExit(1)
}

// Wait waits for all exit handlers to complete.
func AppWait() {
	app_wg.Wait()
}
