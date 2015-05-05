package makecmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// CmdProcess is a wrapper for exec.Cmd that helps to manage
// and monitor its running process.
type CmdProcess struct {
	cmd          *exec.Cmd
	exitChannel  chan error
	exitWait     sync.WaitGroup
	running      bool
	runningMutex sync.Mutex
}

// Start the command process and a goroutine to help manage it.
func (c *CmdProcess) Start() error {
	c.runningMutex.Lock()
	defer c.runningMutex.Unlock()

	if err := c.cmd.Start(); err != nil {
		return err
	}

	c.exitWait.Add(1)
	c.running = true

	// Use a goroutine to wait for the process to exit,
	// and then send the exit status to the exit channel.
	go func() {
		err := c.cmd.Wait()
		c.exitWait.Done()
		c.runningMutex.Lock()
		defer c.runningMutex.Unlock()
		c.running = false
		c.exitChannel <- err
	}()

	return nil
}

// Finished returns a channel that can receive an exit error, indicating
// that it has exited. A nil error means that it exited without error.
func (c *CmdProcess) Finished() chan error {
	return c.exitChannel
}

// IsRunning returns whether the process is running at this point in time.
func (c *CmdProcess) IsRunning() bool {
	c.runningMutex.Lock()
	defer c.runningMutex.Unlock()
	return c.running
}

// Kill the process and wait for it to finish.
func (c *CmdProcess) Kill() error {
	if !c.IsRunning() {
		return nil
	}
	// Operating system specific here. This kills the process and its
	// children. Process.Kill() leaves child processes running on OSX.
	pid := fmt.Sprintf("%d", c.cmd.Process.Pid)
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
func (c *CmdProcess) String() string {
	return strings.Join(c.cmd.Args, " ")
}

// NewCmdProcess initializes a command process.
func NewCmdProcess(name string, args ...string) *CmdProcess {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return &CmdProcess{
		cmd:         cmd,
		exitChannel: make(chan error),
		exitWait:    sync.WaitGroup{},
	}
}
