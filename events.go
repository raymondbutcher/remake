package main

import (
	"log"
	"time"

	"github.com/raymondbutcher/remake/watcher"
)

// makeReadyChannel returns a channel for receiving the ready signal. If there
// are multiple goals, then it returns a dummy channel, as it is not supported.
func makeReadyChannel(goals []string) <-chan bool {
	ready := make(chan bool)
	if len(goals) == 1 {
		// When managing just one target, listen for the ready signal.
		// But don't listen when there are multiple targets, because
		// it wouldn't be known which make process the ready signal
		// is coming from. Unix does support figuring this out,
		// but the Go libraries don't.
		go func() {
			sigchan := ReceiveReadySignal()
			for {
				<-sigchan
				ready <- true
			}
		}()
	}
	return ready
}

// makePollChannel returns a channel for receiving events based on a timer.
// If -poll is not enabled, a dummy channel is returned. A stop function
// is also returned to stop the internal ticker used to send events.
func makePollChannel() (ch <-chan time.Time, stop func()) {
	if pollInterval > 0 {
		ticker := time.NewTicker(pollInterval)
		ch = ticker.C
		stop = ticker.Stop
	} else {
		ch = make(chan time.Time)
		stop = func() {}
	}
	return
}

// makeWatcher sets up and returns a SharedWatcher if -watch is enabled.
func makeWatcher() (w *watcher.SharedWatcher) {
	if watchDebounce > 0 {
		w = watcher.NewSharedWatcher(watchDebounce)
		w.Start()
		go func() {
			for {
				err := <-w.Errors
				log.Printf(red("Watcher error: %s"), err)
			}
		}()
	}
	return
}

// makeWatchChannel returns a channel for receiving filesystem events.
// If -watch is not enabled, a dummy channel is returned.
func makeWatchChannel(wc *watcher.Client) (ch <-chan bool) {
	if wc == nil {
		ch = make(chan bool)
	} else {
		ch = wc.Events
	}
	return
}
