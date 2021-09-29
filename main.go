package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/raymondbutcher/remake/colors"
	"github.com/raymondbutcher/remake/makecmd"
)

const (
	errorSleep = 5 * time.Second
	version    = "0.1.0"
)

var (
	checkInterval time.Duration
	gracePeriod   time.Duration
	readyMode     bool
	versionMode   bool
)

func main() {

	flag.DurationVar(
		&checkInterval,
		"check",
		2*time.Second,
		"Interval between checking for changes",
	)
	flag.DurationVar(
		&gracePeriod,
		"grace",
		10*time.Second,
		"Grace period for commands to finish building",
	)
	flag.BoolVar(
		&readyMode,
		"ready",
		false,
		"Send a ready signal and then quit",
	)
	flag.BoolVar(
		&versionMode,
		"version",
		false,
		"Display the version and then quit",
	)

	flag.Parse()

	if checkInterval <= 0 {
		fmt.Fprintln(os.Stderr, "-check must be non-zero.")
		os.Exit(1)
	}

	if versionMode {
		fmt.Println(version)
		os.Exit(0)
	}

	// Handle when there are no targets in the command line arguments.
	// Remake is consistent with Make in that it will use the default
	// target when no target is specified.
	goals := flag.Args()
	if len(goals) == 0 {
		goals = append(goals, "")
	}

	// If "remake -ready" was run, send the ready signal and then exit.
	if readyMode {
		err := SendReadySignal()
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle signals received from "remake -ready".
	ready := makeReadyChannel(goals)

	// Start managing each goal as a separate goroutine.
	for _, goal := range goals {
		go remake(goal, ready)
	}

	// Block execution forever and let the goroutines work.
	<-make(<-chan struct{})
}

// remake runs the main loop for one make command. It never returns.
func remake(target string, ready <-chan bool) {
	var cmd *makecmd.Cmd
	check, _ := makeCheckChannel()
	for {
		// Create the make command for this target.
		cmd = makecmd.NewCmd(target)

		// Start the command in grace mode. It won't return until
		// it leaves grace mode and it is time for monitoring.
		if err := cmd.StartGraceMode(gracePeriod, ready, check); err != nil {
			log.Printf(colors.Red("Remake: %s"), err)
			time.Sleep(errorSleep)
		} else {
			// And now monitor for changes. It won't return
			// until the make command needs to be restarted.
			cmd.MonitorMode(check)
		}

	}
}

// makeCheckChannel returns a channel that is populated when Remake should
// check for changes. Its behavior depends on the -check option.
func makeCheckChannel() (ch chan struct{}, stop func()) {

	ch = make(chan struct{})

	var checkch <-chan time.Time
	checkch = time.After(checkInterval)

	stopch := make(chan struct{})

	go func() {
		for {
			select {
			case <-checkch:
				ch <- struct{}{}
				checkch = time.After(checkInterval)
			case <-stopch:
				close(ch)
				close(stopch)
				return
			}
		}
	}()

	stop = func() {
		stopch <- struct{}{}
	}

	return
}

// makeReadyChannel returns a channel for receiving the ready signal.
// If there are multiple goals, then it will never receive anything,
// as that is not supported.
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
