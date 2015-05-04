package makedb

import (
	"bufio"
	"bytes"
	"io"
	"log"
)

var (
	defaultGoal = []byte(".DEFAULT_GOAL := ")
)

// readTargets reads from "make --print-data-base" and returns a channel,
// which is populated with blocks of text for each target it finds.
func readTargets(r io.Reader) (ch, dch chan string, done chan struct{}) {

	ch = make(chan string)
	dch = make(chan string)
	done = make(chan struct{})

	go func() {
		defer close(ch)
		defer close(dch)
		defer close(done)

		scanner := bufio.NewScanner(r)

		// Skip ahead to the files section.
		filesHeader := []byte("# Files")
		filesSection := false
		for scanner.Scan() {
			line := scanner.Bytes()
			if bytes.HasPrefix(line, defaultGoal) {
				defaultGoalName := string(line[len(defaultGoal):])
				dch <- defaultGoalName
			} else if bytes.Equal(line, filesHeader) {
				filesSection = true
				break
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
		if !filesSection {
			return
		}

		// Now read each block of text and put them on the channel.
		// Blocks of text are separated by blank links.
		buf := new(bytes.Buffer)
		newline := []byte("\n")
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				if buf.Len() != 0 {
					ch <- buf.String()
					buf = new(bytes.Buffer)
				}
			} else {
				buf.Write(line)
				buf.Write(newline)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
		if buf.Len() != 0 {
			ch <- buf.String()
		}

		done <- struct{}{}

	}()

	return
}
