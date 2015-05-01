package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

var (
	defaultGoal        = []byte(".DEFAULT_GOAL := ")
	doesNotExist       = []byte("#  File does not exist.")
	lastModified       = []byte("#  Last modified ")
	lastModifiedFormat = "2006-01-02 15:04:05"
	needsUpdate        = []byte("#  Needs to be updated (-q is set).")
	notTarget          = []byte("# Not a target:")
	phonyTarget        = []byte("#  Phony target (prerequisite of .PHONY).")
)

// A Database represents a Make database.
type Database struct {
	DefaultGoal string
	Targets     map[string]*Target
}

// NewDatabase returns a Database.
func NewDatabase() Database {
	return Database{
		Targets: map[string]*Target{},
	}
}

// Populate the Database from r, which should contain
// the raw output from "make --print-data-base".
func (db *Database) Populate(r io.Reader) error {
	ch, dch, done := readTargets(r)
	for {
		select {
		case name := <-dch:
			db.DefaultGoal = name
		case s := <-ch:
			t := &Target{}
			if err := t.Populate(s); err != nil {
				return err
			}
			db.Targets[t.Name] = t
		case <-done:
			return nil
		}
	}
}

// TargetQueue is a FIFO queue of unique Targets.
type TargetQueue struct {
	q    []*Target
	seen map[string]bool
}

// NewTargetQueue returns a TargetQueue.
func NewTargetQueue() TargetQueue {
	return TargetQueue{
		q:    []*Target{},
		seen: map[string]bool{},
	}
}

// Push adds a Target to the end of the queue.
// Targets are ignored if they have been previously added.
func (tq *TargetQueue) Push(t *Target) {
	if !tq.seen[t.Name] {
		tq.seen[t.Name] = true
		tq.q = append(tq.q, t)
	}
}

// Pop removes and returns the first Target in the queue.
func (tq *TargetQueue) Pop() (t *Target) {
	t = tq.q[0]
	tq.q = tq.q[1:]
	return
}

// Len returns the number of Targets in the TargetQueue.
func (tq *TargetQueue) Len() int {
	return len(tq.q)
}

// Query checks if a target is OK, or whether it needs updating.
func (db *Database) Query(targetName string, startTime time.Time) (ok bool) {

	var t *Target
	if len(targetName) == 0 {
		t = db.Targets[db.DefaultGoal]
	} else {
		t = db.Targets[targetName]
	}

	if len(t.Name) == 0 {
		panic(t.String())
	}

	var checkTimes bool

	if t.Phony {
		// A phony target, according to Make rules, always needs updating.
		// This does not work with the way that Remake waits for changes.
		// For phony targets, Remake will only check their dependencies.
		// However, phony dependencies will be checked.

		// Anyway, to know when to update a phony target, it is necessary
		// to compare the times of its dependencies againt when the make
		// command was started.
		checkTimes = true
	} else {
		// Real file targets (non-phony) can be checked directly,
		// and don't need to have the file times checked.
		if t.DoesNotExist || t.NeedsUpdate {
			// Needs to be updated.
			ok = false
			return
		}
		checkTimes = false
	}

	// A dependency queue for checking normal prerequisites.
	depQueue := NewTargetQueue()
	for _, name := range t.NormalPrerequisites {
		depQueue.Push(db.Targets[name])
	}

	// An order-only queue for checking order-only prerequisites,
	// which means that only their existance should be checked,
	// and not whether they are up to date.
	orderOnlyQueue := NewTargetQueue()
	for _, name := range t.OrderOnlyPrerequisites {
		orderOnlyQueue.Push(db.Targets[name])
	}

	for depQueue.Len() != 0 || orderOnlyQueue.Len() != 0 {
		for depQueue.Len() != 0 {
			dep := depQueue.Pop()
			if dep.DoesNotExist || dep.NeedsUpdate {
				// Needs to be updated.
				ok = false
				return
			}
			if checkTimes {
				if dep.LastModified.After(startTime) {
					ok = false
					return
				}
			}
			for _, name := range dep.NormalPrerequisites {
				depQueue.Push(db.Targets[name])
			}
			for _, name := range dep.OrderOnlyPrerequisites {
				orderOnlyQueue.Push(db.Targets[name])
			}
		}
		for orderOnlyQueue.Len() != 0 {
			dep := orderOnlyQueue.Pop()
			if dep.DoesNotExist {
				// Needs to be updated.
				ok = false
				return
			}
			for _, name := range dep.NormalPrerequisites {
				depQueue.Push(db.Targets[name])
			}
			for _, name := range dep.OrderOnlyPrerequisites {
				orderOnlyQueue.Push(db.Targets[name])
			}
		}
	}

	// Everything is up to date.
	ok = true
	return
}

// A Target represents a Makefile target.
type Target struct {
	Name                   string
	NormalPrerequisites    []string
	OrderOnlyPrerequisites []string
	NotTarget              bool
	Phony                  bool
	NeedsUpdate            bool
	DoesNotExist           bool
	LastModified           time.Time
}

// PopulateNames populates the name and prerequisites from a line of text.
func (t *Target) PopulateNames(line []byte) error {

	r := bytes.NewReader(line)
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanWords)

	orderOnlyMode := false

	for s.Scan() {
		word := string(s.Bytes())
		if len(t.Name) == 0 {
			t.Name = word[:len(word)-1]
		} else if word[0] == '|' {
			orderOnlyMode = true
		} else if orderOnlyMode {
			t.OrderOnlyPrerequisites = append(t.OrderOnlyPrerequisites, word)
		} else {
			t.NormalPrerequisites = append(t.NormalPrerequisites, word)
		}
	}
	if err := s.Err(); err != nil {
		return err
	}

	if len(t.Name) == 0 {
		return fmt.Errorf("Unable to parse line: %s", line)
	}

	return nil
}

// Populate the target from r, which should contain one
// target's block of text from "make --print-data-base".
func (t *Target) Populate(s string) error {
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := scanner.Bytes()
		if bytes.Equal(line, notTarget) {
			t.NotTarget = true
		} else if len(t.Name) == 0 {
			if err := t.PopulateNames(line); err != nil {
				return err
			}
		} else if bytes.Equal(line, phonyTarget) {
			t.Phony = true
		} else if bytes.Equal(line, needsUpdate) {
			t.NeedsUpdate = true
		} else if bytes.Equal(line, doesNotExist) {
			t.DoesNotExist = true
		} else if bytes.HasPrefix(line, lastModified) {
			s := string(line[len(lastModified):])
			if s == "1970-01-01 00:59:56" {
				panic(fmt.Sprintf("%s: %s", t.String(), line))
			} else {
				lastModified, err := time.ParseInLocation(lastModifiedFormat, s, time.Local)
				if err != nil {
					return err
				}
				t.LastModified = lastModified
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (t Target) String() string {
	status := "ok"
	if t.Phony {
		status = "phony"
	} else if t.DoesNotExist {
		status = "missing"
	} else if t.NeedsUpdate {
		status = "needs update"
	}
	return fmt.Sprintf("%s (%s)", t.Name, status)
}

// readTargets reads from "make --print-data-base" and returns a channel,
// which is populated with blocks of text for each target it finds.
func readTargets(r io.Reader) (ch chan string, dch chan string, done chan struct{}) {

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
