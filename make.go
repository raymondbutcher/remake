package main

import (
	"bytes"
	"log"
	"os/exec"
	"time"

	"github.com/raymondbutcher/remake/makedb"
)

// MakeCommand is used to manage a make command, its running process,
// and checking if it is up to date.
type MakeCommand struct {
	Target      string
	cmd         *Cmd
	cmdArgs     []string
	queryArgs   []string
	db          *makedb.Database
	progressed  time.Time
	remaining   int
	usedChanged bool
}

// NewMakeCommand initializes a make command.
func NewMakeCommand(target string) *MakeCommand {
	mc := MakeCommand{
		cmdArgs: []string{
			"--warn-undefined-variables",
		},
		queryArgs: []string{
			"--warn-undefined-variables",
			"--question",
			"--print-data-base",
		},
	}
	if len(target) != 0 {
		mc.Target = target
		mc.cmdArgs = append(mc.cmdArgs, target)
		mc.queryArgs = append(mc.queryArgs, target)
	}
	return &mc
}

// GetFiles gets the filenames of the target and its dependencies.
func (mc *MakeCommand) GetFiles() (names []string) {
	// Use the last known database to avoid running make again.
	if mc.db == nil {
		mc.db = mc.getDatabase()
	}
	add := func(t *makedb.Target) {
		if !t.Phony {
			names = append(names, t.Name)
		}
	}
	t := mc.db.GetTarget(mc.Target)
	add(t)
	nDeps, oDeps := mc.db.GetDeps(t.Name)
	for _, name := range nDeps {
		add(mc.db.GetTarget(name))
	}
	for _, name := range oDeps {
		add(mc.db.GetTarget(name))
	}
	return
}

// Finished returns a channel that can receive an exit error, indicating
// that it has exited. A nil error means it exited without error.
func (mc *MakeCommand) Finished() chan error {
	return mc.cmd.Finished()
}

// HasChanged checks if the make target has changed since Progress was
// last called. It is subtle, but Progress should be used during "grace
// mode" to find out when the make command has finished building itself
// and its dependencies. Afterwards, HasChanged should be used to check
// if the command should be restarted due to new changes.
func (mc *MakeCommand) HasChanged() bool {

	if mc.progressed.IsZero() {
		panic("Cannot use HasChanged before UpdateProgress")
	}
	if !mc.usedChanged {
		mc.usedChanged = true
	}

	db := mc.getDatabase()
	t := db.GetTarget(mc.Target)

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
			return true
		}
		checkTimes = false
	}

	nDeps, oDeps := db.GetDeps(t.Name)

	for _, name := range nDeps {
		dep := db.GetTarget(name)
		if dep.DoesNotExist || dep.NeedsUpdate {
			return true
		} else if checkTimes {
			if dep.LastModified.After(mc.progressed) {
				return true
			}
		}
	}

	for _, name := range oDeps {
		dep := db.GetTarget(name)
		if dep.DoesNotExist {
			return true
		}
	}

	return false
}

// Kill the command and wait for it to finish.
func (mc *MakeCommand) Kill() {
	for {
		if err := mc.cmd.Kill(); err != nil {
			log.Printf(red("Remake: Error killing %s: %s"), mc.String(), err)
			time.Sleep(1 * time.Second)
		} else {
			return
		}
	}
}

// UpdateProgress checks how many targets need updating and stores it.
// It also updates the internal time used by HasChanged.
func (mc *MakeCommand) UpdateProgress() {
	if mc.usedChanged {
		panic("Cannot use UpdateProgress after HasChanged")
	}
	mc.progressed = time.Now()
	mc.remaining = mc.getRemaining()
}

// CheckProgress returns the number of targets that need to be updated. This
// is used during grace mode to check if a make command is making progress
// with building its dependencies. Always use UpdateProgress before using
// CheckProgress. They are separate methods only for clarity of purpose.
func (mc *MakeCommand) CheckProgress() (remaining int) {
	return mc.remaining
}

// Start the make command.
func (mc *MakeCommand) Start() error {
	mc.cmd = NewCommand("make", mc.cmdArgs...)
	return mc.cmd.Start()
}

// String returns the underlying make command that gets run.
func (mc *MakeCommand) String() string {
	return mc.cmd.String()
}

// getDatabase runs the make query for this make target
// and populates a new database with the results.
func (mc *MakeCommand) getDatabase() *makedb.Database {
	cmd := exec.Command("make", mc.queryArgs...)
	out, _ := cmd.Output()
	r := bytes.NewReader(out)
	db := makedb.NewDatabase()
	err := db.Populate(r)
	if err != nil {
		panic(err)
	}
	mc.db = &db
	return &db
}

// getRemaining counts how many targets need to be updated
// for this make command's target to be considered up to date.
func (mc *MakeCommand) getRemaining() (count int) {
	db := mc.getDatabase()
	t := db.GetTarget(mc.Target)
	if !t.Phony {
		// Real file targets (non-phony) can be checked directly.
		if t.DoesNotExist || t.NeedsUpdate {
			count++
		}
	}
	nDeps, oDeps := db.GetDeps(t.Name)
	for _, name := range nDeps {
		dep := db.GetTarget(name)
		if dep.DoesNotExist || dep.NeedsUpdate {
			count++
		}
	}
	for _, name := range oDeps {
		dep := db.GetTarget(name)
		if dep.DoesNotExist {
			count++
		}
	}
	return
}
