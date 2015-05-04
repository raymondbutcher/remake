package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/raymondbutcher/remake/watcher"
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
func remake(target string, ready <-chan bool, w *watcher.SharedWatcher) {
	var (
		cmd *MakeCommand
		err error
		wc  *watcher.Client
	)
	if w != nil {
		wc = w.NewClient()
	}
	for {
		cmd = NewMakeCommand(target)

		// Start the command and run in grace mode. Use a lock to prevent
		// multiple make commands starting up at the same time. Otherwise,
		// separate make commands with shared dependencies would be able
		// to build the same targets at the same time.
		buildMutex.Lock()
		err = cmd.Start()
		if err == nil {
			err = GraceMode(cmd, ready, wc)
		}
		buildMutex.Unlock()

		if err == nil {
			MonitorMode(cmd, wc)
		} else {
			log.Printf(red("Remake: %s"), err)
			time.Sleep(remakeErrorSleep)
		}
	}
}

// updateWatchedFiles adds the make command's target files to the watcher.
// This is called regularly to ensure it stays up to date (e.g a makefile
// could have wildcards that find new files). If -watch is not enabled,
// this does nothing.
func updateWatchedFiles(wc *watcher.Client, cmd *MakeCommand) {
	if wc != nil {
		// Add directories rather than files. This helps to catch makefile
		// targets with wildcards and "find" shell commands. The fsnotify
		// library does not support recursive watching, and I have hit open
		// file limits doing that manually, but this approach will probably
		// work adequately.
		dirs := map[string]bool{}
		for _, name := range cmd.GetFiles() {
			if fi, err := os.Stat(name); err == nil && !fi.IsDir() {
				name = filepath.Dir(name)
			}
			if _, seen := dirs[name]; !seen {
				dirs[name] = true
				if err := wc.Watcher.Add(name); err != nil {
					log.Printf(yellow("Error watching directory '%s': %s"), name, err)
				}
			}
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
