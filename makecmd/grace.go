package makecmd

import (
	"fmt"
	"sync"
	"time"
)

// Use a lock to prevent multiple make commands starting up at the same
// time. Otherwise, separate make commands with shared dependencies would
// be able to build the same targets at the same time.
var buildMutex sync.Mutex

// progressChecker is used to keep track of the make command's
// build progress when running in grace mode.
type progressChecker struct {
	stalled   <-chan time.Time
	cmd       *Cmd
	grace     time.Duration
	remaining int
}

func newProgressChecker(cmd *Cmd, gracePeriod time.Duration) progressChecker {
	return progressChecker{
		stalled: time.After(gracePeriod),
		cmd:     cmd,
		grace:   gracePeriod,
	}
}

func (pc progressChecker) check() (done, progressing bool) {
	pc.cmd.UpdateProgress()
	rem := pc.cmd.CheckProgress()
	done = (rem == 0)
	progressing = (rem != pc.remaining)
	pc.remaining = rem
	if progressing && !done {
		pc.extendGraceMode()
	}
	return
}

func (pc progressChecker) extendGraceMode() {
	pc.stalled = time.After(pc.grace)
}

// StartGraceMode starts the command and monitors it as it starts up,
// waiting for it to finish updating anything required.
func (cmd *Cmd) StartGraceMode(
	gracePeriod time.Duration,
	readyChannel <-chan bool,
	checkChannel <-chan struct{},
) error {

	// Limit commands running in grace mode to 1 at a time.
	buildMutex.Lock()
	defer buildMutex.Unlock()

	if err := cmd.cmd.Start(); err != nil {
		return fmt.Errorf("Error starting %s: %s", cmd, err)
	}

	// Keep track of whether the make command is making progress, or if it
	// seems to be doing nothing. If there is no discernable progress for
	// a length of time exceeding the grace period, then the command will
	// be killed, to be restarted by the calling function.
	progress := newProgressChecker(cmd, gracePeriod)

	for {
		select {
		case <-readyChannel:
			// A signal has been sent by "remake -ready" so leave grace mode.
			// Also, update progress to ensure that the monitor mode checks
			// timestamps against now onwards.
			cmd.UpdateProgress()
			return nil

		case <-cmd.cmd.Finished():
			// The command has exited already, so leave grace mode.
			// Also, update progress to ensure that the monitor mode checks
			// timestamps against now onwards.
			cmd.UpdateProgress()
			return nil

		case <-checkChannel:
			if done, _ := progress.check(); done {
				return nil
			}

		case <-progress.stalled:
			// No progress has been made for some time.
			// Give it one last chance before killing it.
			if done, progressed := progress.check(); done {
				// Valid scenario that gets here: a long-running-process
				// phony target, already up to date, doesn't use the
				// "remake -ready" signal, checking disabled. (but that is not possible now!)
				return nil
			} else if progressed {
				continue
			}
			cmd.mustKill()
			return fmt.Errorf("grace period exceeded: %s", cmd)
		}
	}
}
