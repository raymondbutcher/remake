package makedb

import (
	"fmt"
	"io"
	"time"
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

// GetDeps finds and returns the chain of dependencies for a target.
// Results are split into 2 lists: normal prerequisites, and order-only
// prerequisites (which should be checked for existence only).
func (db *Database) GetDeps(targetName string) (normal []string, orderOnly []string) {

	target, found := db.Targets[targetName]
	if !found {
		panic(fmt.Sprintf("Target '%s' not found", targetName))
	}

	nq := NewUniqueQueue()
	for _, name := range target.NormalPrerequisites {
		nq.Push(name)
	}

	oq := NewUniqueQueue()
	for _, name := range target.OrderOnlyPrerequisites {
		oq.Push(name)
	}

	for nq.Len() != 0 {
		name := nq.Pop()
		normal = append(normal, name)
		dep := db.GetTarget(name)
		for _, name := range dep.NormalPrerequisites {
			nq.Push(name)
		}
		for _, name := range dep.OrderOnlyPrerequisites {
			oq.Push(name)
		}
	}

	for oq.Len() != 0 {
		name := oq.Pop()
		normal = append(normal, name)
		dep := db.GetTarget(name)
		for _, name := range dep.NormalPrerequisites {
			// Normal prerequisites of order-only prerequesites remain
			// as order-only prerequisites for the original target.
			oq.Push(name)
		}
		for _, name := range dep.OrderOnlyPrerequisites {
			oq.Push(name)
		}
	}

	return
}

// GetTarget returns a Target, or panics if it can't.
func (db *Database) GetTarget(name string) (t *Target) {
	if len(name) == 0 {
		t = db.Targets[db.DefaultGoal]
	} else {
		t = db.Targets[name]
	}
	if len(t.Name) == 0 {
		panic(fmt.Sprintf("Target '%s' not found", name))
	}
	return
}

func (db *Database) GetPendingTargets(target string, since time.Time) (count int) {
	// For the specified target, return the number of targets (including itself
	// and its dependencies) that are missing or need to be updated.

	t := db.GetTarget(target)

	// Check the specified target.
	if !t.Phony && (t.DoesNotExist || t.NeedsUpdate) {
		count++
	}

	nDeps, oDeps := db.GetDeps(t.Name)

	// Phony targets, according to the Make database, always needs updating.
	// This does not work with the way that Remake waits for changes.
	// For phony targets, Remake will only check their dependencies
	// and restart when real file targets (non-phony) dependencies
	// have changed.
	foundNewer := false

	// Check the target's normal prerequisites.
	for _, name := range nDeps {
		dep := db.GetTarget(name)
		if !dep.Phony {
			if dep.DoesNotExist || dep.NeedsUpdate {
				count++
			} else if t.Phony && dep.LastModified.After(since) {
				foundNewer = true
			}
		}
	}

	if foundNewer {
		count++
	}

	// Check the target's order-only prerequisites.
	// This type only needs to exist (if it's not a phony target).

	for _, name := range oDeps {
		dep := db.GetTarget(name)
		if !dep.Phony && dep.DoesNotExist {
			count++
		}
	}

	return
}
