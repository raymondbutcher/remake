package main

import "testing"

func ExampleCmd() {
	cmd := NewCommand("echo", "hello from echo")
	cmd.Start()
	<-cmd.Finished()
	// Output: hello from echo
}

func TestCmd(t *testing.T) {
	// Start a long-running process and then kill it.
	cmd := NewCommand("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Could not start command: %s", err)
	}
	if !cmd.IsRunning() {
		t.Fatal("Expected it to be running.")
	}
	if err := cmd.Kill(); err != nil {
		t.Fatalf("Error during Kill: %s", err)
	}
	select {
	case <-cmd.Finished():
	default:
		t.Error("Finished channel was empty.")
	}
}
