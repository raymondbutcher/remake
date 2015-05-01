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
	Cmd         *exec.Cmd
	exitChannel chan error
	exitWait    sync.WaitGroup
}

// Start the command process and a goroutine to help manage it.
func (c *Cmd) Start() error {

	c.exitWait.Add(1)

	if err := c.Cmd.Start(); err != nil {
		return err
	}

	// Use a goroutine to wait for the process to exit,
	// and then send the exit status to the exit channel.
	go func() {
		err := c.Cmd.Wait()
		c.exitWait.Done()
		c.exitChannel <- err
	}()

	return nil
}

// Finished returns a channel that can receive an exit error, indicating
// that it has exited. A nil error means it exited without error.
func (c *Cmd) Finished() chan error {
	return c.exitChannel
}

// Kill the command and wait for it to finish.
func (c *Cmd) Kill() error {
	// Operating system specific here. This kills the process and its
	// children. Process.Kill() leaves child processes running on OSX.
	pid := fmt.Sprintf("%d", c.Cmd.Process.Pid)
	kill := exec.Command("kill", pid)
	kill.Stdout = os.Stdout
	kill.Stderr = os.Stderr
	if err := kill.Run(); err != nil {
		return err
	}
	c.exitWait.Wait()
	return nil
}

// String returns the underlying command.
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
