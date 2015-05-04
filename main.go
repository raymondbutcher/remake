package main

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/raymondbutcher/remake/fswatch"
	"github.com/raymondbutcher/remake/makecmd"
)

const (
	remakeErrorSleep = 5 * time.Second
	exitErrorSleep   = 5 * time.Second
)

var (
	buildMutex sync.Mutex
)

func main() {
	// Parse and validate the command line options.
	goals, readyMode := processArguments()

	// If "remake -ready" was run, send the ready signal and then exit.
	if readyMode {
		err := SendReadySignal()
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle signals received from "remake -ready".
	ready := makeReadyChannel(goals)

	// Handle events from the filesystem.
	w := makeWatcher()

	// Start managing each goal as a separate goroutine.
	for _, goal := range goals {
		go remake(goal, ready, w)
	}

	// Block execution forever and let the goroutines work.
	<-make(chan struct{})
}

// remake runs the main loop for one make command.
func remake(target string, ready <-chan bool, watcher *fswatch.SharedWatcher) {
	var (
		cmd           *makecmd.Cmd
		err           error
		watcherClient *fswatch.Client
	)
	if watcher != nil {
		watcherClient = watcher.NewClient()
	}
	for {
		cmd = makecmd.NewCmd(target)

		// Start the command and run in grace mode. Use a lock to prevent
		// multiple make commands starting up at the same time. Otherwise,
		// separate make commands with shared dependencies would be able
		// to build the same targets at the same time.
		buildMutex.Lock()
		err = cmd.Start()
		if err == nil {
			err = GraceMode(cmd, ready, watcherClient)
		}
		buildMutex.Unlock()

		if err == nil {
			MonitorMode(cmd, watcherClient)
		} else {
			log.Printf(red("Remake: %s"), err)
			time.Sleep(remakeErrorSleep)
		}
	}
}

func red(s string) string {
	const (
		color = "\033[0;31m"
		reset = "\033[0m"
	)
	return color + s + reset
}

func yellow(s string) string {
	const (
		color = "\033[0;33m"
		reset = "\033[0m"
	)
	return color + s + reset
}
