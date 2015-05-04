package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// SignalListener has channels and methods required to watch signals.
type SignalListener struct {
	recv chan os.Signal
	stop chan bool
	done chan bool
}

// NewSignalListener starts listening for a signal,
// and returns a Listener for management.
func NewSignalListener() *SignalListener {
	return &SignalListener{
		recv: make(chan os.Signal, 1),
		stop: make(chan bool),
		done: make(chan bool, 1),
	}
}

// Listen starts listening for a signal type
// and returns the channel for receiving them.
func (l *SignalListener) Listen(sig os.Signal) chan os.Signal {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, sig)
	go func() {
		for {
			select {
			case sig := <-sigchan:
				if sig == nil {
					l.done <- true
					return
				}
				l.recv <- sig
			case <-l.stop:
				signal.Stop(sigchan)
				close(sigchan)
			}
		}
	}()
	return l.recv
}

// Stop will stop listening for the signal.
func (l *SignalListener) Stop() {
	l.stop <- true
	<-l.done
}

// ReceiveReadySignal listens for "ready" signals,
// and returns a channel for receiving them.
func ReceiveReadySignal() chan os.Signal {
	l := NewSignalListener()
	return l.Listen(syscall.SIGUSR1)
}

// SendReadySignal tries to send a "ready" signal
// to the ancestor Remake process, if there is one.
func SendReadySignal() (err error) {
	processID := os.Getpid()
	processName, err := getProcessName(processID)
	if err != nil {
		panic(err)
	}

	// Search for an ancestor process with the same name as this one. In other
	// words, find the original "remake" process that ran "remake -ready".
	parentID := os.Getppid()
	for {
		if parentID == 0 {
			return nil
		}
		name, err := getProcessName(parentID)
		if err != nil {
			panic(err)
		}
		if name == processName {
			break
		}
		parentID, err = getParentID(parentID)
		if err != nil {
			panic(err)
		}
	}

	// The ancestor process has been found, so it can be signaled. That lets
	// it know that the dependencies have been built, and it can proceed past
	// the init stage and start monitoring for changes.
	p, err := os.FindProcess(parentID)
	if err != nil {
		return err
	}

	if err := p.Signal(syscall.SIGUSR1); err != nil {
		panic(err)
	}

	return nil
}

// getProcessName gets the base name of a process.
func getProcessName(pid int) (name string, err error) {
	p := fmt.Sprintf("%d", pid)
	cmd := exec.Command("ps", "-p", p, "-o", "comm=")
	out, err := cmd.Output()
	if err != nil {
		return name, err
	}
	name = filepath.Base(string(out))
	return name, nil
}

// getParentID gets the parent ID of a process.
func getParentID(pid int) (ppid int, err error) {
	spid := fmt.Sprintf("%d", pid)
	out, err := exec.Command("ps", "-p", spid, "-o", "ppid=").Output()
	if err != nil {
		return ppid, err
	}
	ppid, err = strconv.Atoi(strings.TrimSpace(string(out)))
	return ppid, err
}
