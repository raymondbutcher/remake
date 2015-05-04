package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

var (
	gracePeriod   time.Duration
	pollInterval  time.Duration
	watchDebounce time.Duration
)

func processArguments() (goals []string, readyMode bool) {
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
	goals = flag.Args()
	if len(goals) == 0 {
		goals = append(goals, "")
	}

	return
}
