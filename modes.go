package main

import (
	"fmt"
	"log"
	"time"

	"github.com/raymondbutcher/remake/watcher"
)

type progressChecker struct {
	cmd       *MakeCommand
	stalled   <-chan time.Time
	remaining int
}

func newProgressChecker(cmd *MakeCommand) progressChecker {
	return progressChecker{
		cmd:     cmd,
		stalled: time.After(gracePeriod),
	}
}

func (pc progressChecker) check() (done, progressing bool) {
	pc.cmd.UpdateProgress()
	rem := pc.cmd.CheckProgress()
	done = (rem == 0)
	progressing = (rem != pc.remaining)
	pc.remaining = rem
	if progressing && !done {
		// Things are progressing, so extend grace mode.
		pc.extend()
	}
	return
}

func (pc progressChecker) extend() {
	pc.stalled = time.After(gracePeriod)
}

// GraceMode monitors the make command as it starts up, waiting for it to
// finish updating.
func GraceMode(cmd *MakeCommand, ready <-chan bool, wc *watcher.Client) error {
	// Keep track of whether the make command is making progress, or if it
	// seems to be doing nothing. If there is no discernable progress for
	// a length of time exceeding the grace period, then the command will
	// be killed, to be restarted by the calling function.
	progress := newProgressChecker(cmd)

	// A long-running-process phony target with already-up-to-date
	// dependencies, which doesn't use "remake -ready", should leave
	// grace mode immediately. There will be no filesystem events to
	// trigger a check, so force a check to happen after 1 second.
	forcedCheck := time.After(1 * time.Second)

	pollCheck, pollStop := makePollChannel()
	defer pollStop()

	watchCheck := makeWatchChannel(wc)

	check := make(chan bool, 1)

	for {
		select {
		case <-ready:
			// A signal has been sent by "remake -ready" so leave grace mode.
			// Also, update progress to ensure that the monitor mode checks
			// timestamps from now onwards.
			cmd.UpdateProgress()
			return nil

		case err := <-cmd.Finished():
			// The command exited already. If it returned an error exit status,
			// then just log it. Either way, success or error, leave grace mode.
			// Also, update progress to ensure that the monitor mode checks
			// timestamps from now onwards.
			cmd.UpdateProgress()
			if err != nil {
				log.Printf(yellow("%s: %s"), cmd, err)
			}
			return nil

		case <-forcedCheck:
			check <- true
		case <-pollCheck:
			check <- true
		case <-watchCheck:
			log.Println("file event in grace mode")
			check <- true
		case <-check:
			if done, _ := progress.check(); done {
				return nil
			}
			updateWatchedFiles(wc, cmd)

		case <-progress.stalled:
			// No progress has been made for some time.
			// Give it one last chance before killing it.
			if done, progressed := progress.check(); done {
				return nil
			} else if progressed {
				continue
			}
			cmd.Kill()
			return fmt.Errorf("Grace period exceeded: %s", cmd)
		}
	}
}

// MonitorMode monitors the make command's target to see if it needs updating.
// If it does, and the command is still running, then it will kill the command.
// It will not return until it needs updating and it is not running.
func MonitorMode(cmd *MakeCommand, wc *watcher.Client) {

	pollCheck, pollStop := makePollChannel()
	defer pollStop()

	watchCheck := makeWatchChannel(wc)
	updateWatchedFiles(wc, cmd)

	check := make(chan bool, 1)

	for {
		select {
		case err := <-cmd.Finished():
			// The command exited. If it returned an error exit status, then
			// just log it. Either way, success or error, don't actually do
			// anything here because this doesn't mean that the make target
			// needs updating.
			if err != nil {
				log.Printf(yellow("%s: %s"), cmd, err)
			}

		case <-pollCheck:
			check <- true
		case <-watchCheck:
			check <- true
		case <-check:
			if cmd.HasChanged() {
				// The make target is no longer up to date. Kill the process
				// if it is still running, and then return so the make command
				// can be started again.
				cmd.Kill()
				return
			}
			updateWatchedFiles(wc, cmd)
		}
	}
}
