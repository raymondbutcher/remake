package makedb

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"time"
)

var (
	doesNotExist       = []byte("#  File does not exist.")
	lastModified       = []byte("#  Last modified ")
	lastModifiedFormat = "2006-01-02 15:04:05"
	needsUpdate        = []byte("#  Needs to be updated (-q is set).")
	notTarget          = []byte("# Not a target:")
	phonyTarget        = []byte("#  Phony target (prerequisite of .PHONY).")
)

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
