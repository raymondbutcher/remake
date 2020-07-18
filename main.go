package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"./colors"
	"./fswatch"
	"./makecmd"
)

const (
	errorSleep = 5 * time.Second
)

var (
	readyMode     bool
	gracePeriod   time.Duration
	pollInterval  time.Duration
	watchDebounce time.Duration
)

func main() {

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
		"Grace period for commands to finish building",
	)
	flag.DurationVar(
		&pollInterval,
		"poll",
		0*time.Second,
		"Regularly poll for changes",
	)
	flag.DurationVar(
		&watchDebounce,
		"watch",
		100*time.Millisecond,
		"Debounce time for watching the filesystem",
	)

	// Show poll=0s rather than poll=0 in the command line help.
	flag.CommandLine.Lookup("poll").DefValue = "0s"

	flag.Parse()

	if watchDebounce <= 0 && pollInterval <= 0 {
		fmt.Fprintln(os.Stderr, "-watch or -poll must be enabled.")
		os.Exit(1)
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

	// Handle events from the filesystem.
	watcher := makeWatcher()

	// Start managing each goal as a separate goroutine.
	for _, goal := range goals {
		go remake(goal, ready, watcher)
	}

	// Block execution forever and let the goroutines work.
	<-make(<-chan struct{})
}

// remake runs the main loop for one make command. It never returns.
func remake(target string, ready <-chan bool, watcher *fswatch.SharedWatcher) {
	var (
		cmd      *makecmd.Cmd
		watcherc *fswatch.Client
	)
	if watcher != nil {
		watcherc = watcher.NewClient()
	}
	check, _ := makeCheckChannel(watcherc)
	for {
		// Create the make command for this target.
		cmd = makecmd.NewCmd(target)

		// Start watching for filesystem events before starting the command.
		// Watch directories of target files, rather than just files, because
		// there might be wildcard targets, and new files would not watched
		// if only watching the known targets.
		if watcherc != nil {
			for _, name := range cmd.GetFiles() {
				watcherc.Watcher.AddDir(name)
			}
		}

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
// check for changes. Its behavior depends on the -poll and -watch options.
func makeCheckChannel(watcherc *fswatch.Client) (ch chan struct{}, stop func()) {

	ch = make(chan struct{})

	var pch <-chan time.Time
	if pollInterval > 0 {
		pch = time.After(pollInterval)
	}

	var fch <-chan bool
	if watcherc != nil {
		fch = watcherc.C
	}

	stopch := make(chan struct{})

	go func() {
		for {
			select {
			case <-pch:
				ch <- struct{}{}
				pch = time.After(pollInterval)
			case <-fch:
				ch <- struct{}{}
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

// makeWatcher sets up and returns a SharedWatcher if -watch is enabled.
// It automatically watches the current working directory, so that any
// changes to the makefile will trigger checks.
func makeWatcher() (watcher *fswatch.SharedWatcher) {
	if watchDebounce > 0 {
		watcher = fswatch.NewSharedWatcher(watchDebounce)
		watcher.Start()
		if cwd, err := os.Getwd(); err != nil {
			log.Fatalf("os.Getwd(): %s", err)
		} else if err := watcher.Add(cwd); err != nil {
			log.Fatalf("watcher.Add(%s): %s", cwd, err)
		}
		go func() {
			for {
				err := <-watcher.Errors
				log.Printf(colors.Red("Watcher error: %s"), err)
			}
		}()
	}
	return
}
