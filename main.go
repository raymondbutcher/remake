package main

import (
	"flag"
	"log"
	"os"
	"sync"
	"time"
)

const (
	checkInterval    = 2 * time.Second
	remakeErrorSleep = 5 * time.Second
	exitErrorSleep   = 5 * time.Second
)

var (
	gracePeriod time.Duration
	readyMode   bool
	buildMutex  sync.Mutex
)

func main() {
	// Parse the command line arguments.
	flag.BoolVar(
		&readyMode,
		"ready",
		false,
		"Send a ready signal and then quit",
	)
	flag.DurationVar(
		&gracePeriod,
		"grace",
		10*time.Second,
		"Grace period for a command to start up",
	)
	flag.Parse()

	// If "remake -ready" was run, send the ready signal and then exit.
	if readyMode {
		err := SendReadySignal()
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle when there is no target in the command line arguments.
	// Remake will use the default target if no target is specified.
	goals := flag.Args()
	if len(goals) == 0 {
		goals = append(goals, "")
	}

	// Handle signals from "remake -ready".
	var ready chan (os.Signal)
	if len(goals) == 1 {
		// When managing one make command, listen for the signal.
		ready = ReceiveReadySignal()
	} else {
		// When there are multiple make commands, and a ready signal is
		// received, which make command was it from? Unix does support
		// figuring this out, but the Go library doesn't. So the ready
		// signals will be ignored in this case. Use a dummy channel
		// that will never receive a signal.
		ready = make(chan os.Signal)
	}

	// And so it begins. Start one goroutine per goal.
	for _, goal := range goals {
		go remake(goal, ready)
	}

	// Block execution forever and let the goroutines work.
	block := make(chan bool)
	<-block
}

// remake runs the main loop for one make command.
func remake(goal string, ready chan os.Signal) {
	// Use an infinite loop. The manageMake starts a make command, and
	// will only return when it needs to be started again, for any reason.
	for {
		manageMake(goal, ready)
	}
}

// manageMake runs a make command, watches for changes, and kills the command
// if/when changes are found. It returns only when it should be started again.
func manageMake(goal string, ready chan os.Signal) {
	// Prevent multiple make commands from starting up at the same time.
	// Otherwise, separate make commands with shared dependencies could
	// build the same targets at the same time. This lock will last until
	// either the command exits, or it leaves grace mode and starts the
	// normal monitoring behavior.
	buildMutex.Lock()
	locked := true
	unlock := func() {
		if locked {
			buildMutex.Unlock()
			locked = false
		}
	}
	defer unlock()

	// Keep track of when the make command was run. This will be changed
	// to be just after the grace mode finishes, if the command doesn't
	// finish first, so that changes made by the command won't trigger
	// itself to be restarted. This only affects make commands for
	// phony targets.
	startTime := time.Now()

	// Run the underlying make command.
	var makeCmd *Cmd
	if len(goal) == 0 {
		makeCmd = NewCommand("make")
	} else {
		makeCmd = NewCommand("make", goal)
	}
	if err := makeCmd.Start(); err != nil {
		// It failed to start.
		log.Printf(red("Remake: %s: %s"), makeCmd.String(), err)
		time.Sleep(remakeErrorSleep)
		// Return so it will try to run the command again.
		return
	}

	// Create the query that can check for changes.
	query := NewQuery(goal)

	// Run in grace mode at first.
	running := true
	graceMode := true
	graceModeEnd := time.After(gracePeriod)
	leaveGraceMode := func() {
		graceMode = false
		unlock()
	}

	// Start monitoring.
	for {
		select {
		case <-ready:
			// Ready signal received.
			startTime = time.Now()
			leaveGraceMode()

		case err := <-makeCmd.Finished():
			// Command finished.
			if err != nil {
				log.Printf(red("Remake %s: %s"), err, makeCmd.String())
			}
			running = false
			leaveGraceMode()

		case <-time.After(checkInterval):
			// Regularly check for changes.
			changed, err := query.Run(startTime)
			if err != nil {
				// Error running the query to check for changes.
				// Sleep and let it try again.
				log.Printf(red("Remake %s: %s"), err, query.String())
				time.Sleep(remakeErrorSleep)
			} else if graceMode {
				if changed {
					// Push the start time forward. Trying to minimize
					// the window for a race condition to occur.
					startTime = time.Now()
				} else {
					// Everything is up to date, so it must be ready to
					// leave grace mode and start monitoring normally.
					leaveGraceMode()
				}
			} else if changed {
				// Detected changes outside of grace mode.
				// Kill the process if it is still running.
				if running {
					if err := makeCmd.Kill(); err != nil {
						log.Printf(red("Remake: kill: %s"), err)
					}
				}
				// Return so it can run the command again.
				return
			}

		case <-graceModeEnd:
			if graceMode && running {
				// This might happen due to a race condition when changing
				// files rapidly. The "remake --ready" command was created
				// to avoid this, and it should not allow this to happen
				// unless builds actually take this long to finish running.
				log.Printf(
					red("Remake initializing for too long: %s"),
					makeCmd.String(),
				)
				makeCmd.Kill()
				// Return so it can run the command again.
				return
			}
		}
	}
}

// red makes text red.
func red(s string) string {
	const (
		redColor   = "\033[0;31m"
		resetColor = "\033[0m"
	)
	return redColor + s + resetColor
}
