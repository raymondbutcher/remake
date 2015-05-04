package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Cmd is a wrapper for exec.Cmd, adding channels
// and methods to help monitor and manage it.
type Cmd struct {
	Cmd          *exec.Cmd
	exitChannel  chan error
	exitWait     sync.WaitGroup
	running      bool
	runningMutex sync.Mutex
}

// Start the command process and a goroutine to help manage it.
func (c *Cmd) Start() error {
	c.runningMutex.Lock()
	defer c.runningMutex.Unlock()

	c.exitWait.Add(1)

	if err := c.Cmd.Start(); err != nil {
		return err
	}

	c.running = true

	// Use a goroutine to wait for the process to exit,
	// and then send the exit status to the exit channel.
	go func() {
		err := c.Cmd.Wait()
		c.exitWait.Done()
		c.runningMutex.Lock()
		defer c.runningMutex.Unlock()
		c.running = false
		c.exitChannel <- err
	}()

	return nil
}

// Finished returns a channel that can receive an exit error, indicating
// that it has exited. A nil error means it exited without error.
func (c *Cmd) Finished() chan error {
	return c.exitChannel
}

// IsRunning returns whether the command is running at this point in time.
func (c *Cmd) IsRunning() bool {
	c.runningMutex.Lock()
	defer c.runningMutex.Unlock()
	return c.running
}

// Kill the command and wait for it to finish.
func (c *Cmd) Kill() error {
	if !c.IsRunning() {
		return nil
	}
	// Operating system specific here. This kills the process and its
	// children. Process.Kill() leaves child processes running on OSX.
	pid := fmt.Sprintf("%d", c.Cmd.Process.Pid)
	kill := exec.Command("kill", pid)
	kill.Stdout = os.Stdout
	kill.Stderr = os.Stderr
	err := kill.Run()
	if err == nil {
		c.exitWait.Wait()
	}
	return err
}

// String returns the underlying command that gets run.
func (c *Cmd) String() string {
	return strings.Join(c.Cmd.Args, " ")
}

// NewCommand creates and returns a Cmd.
func NewCommand(name string, args ...string) *Cmd {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return &Cmd{
		Cmd:         c,
		exitChannel: make(chan error),
		exitWait:    sync.WaitGroup{},
	}
}
