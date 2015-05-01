package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Query defines a Makefile query that can be run.
type Query struct {
	goal string
	args []string
}

// NewQuery initializes a Query.
func NewQuery(goal string) *Query {
	args := []string{
		"--question",
		"--print-data-base",
		"--warn-undefined-variables",
	}
	if len(goal) != 0 {
		args = append(args, goal)
	}
	return &Query{
		goal: goal,
		args: args,
	}
}

// Run the query to see if anything has changed.
func (q *Query) Run(startTime time.Time) (changed bool, err error) {
	// Run Make to print out all of the info.
	cmd := exec.Command("make", q.args...)
	cmd.Stderr = os.Stderr
	out, _ := cmd.Output()

	// Read that into a database.
	r := bytes.NewReader(out)
	db := NewDatabase()
	if err := db.Populate(r); err != nil {
		return false, err
	}

	ok := db.Query(q.goal, startTime)
	return !ok, nil
}

func (q Query) String() string {
	return "make " + strings.Join(q.args, " ")
}
