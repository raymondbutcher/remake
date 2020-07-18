package makecmd

import (
	"bytes"
	"log"
	"os/exec"
	"time"

	"../colors"
	"../makedb"
)

// Cmd is used to manage a make command, its running process,
// and to check if its target is up to date.
type Cmd struct {
	Target      string
	cmd         *CmdProcess
	queryArgs   []string
	db          *makedb.Database
	progressed  time.Time
	remaining   int
	usedChanged bool
}

// NewCmd initializes a make command.
func NewCmd(target string) *Cmd {
	cmdArgs := []string{
		"--warn-undefined-variables",
	}
	queryArgs := []string{
		"--warn-undefined-variables",
		"--question",
		"--print-data-base",
	}
	if len(target) != 0 {
		cmdArgs = append(cmdArgs, target)
		queryArgs = append(queryArgs, target)
	}
	return &Cmd{
		Target:    target,
		cmd:       NewCmdProcess("make", cmdArgs...),
		queryArgs: queryArgs,
	}
}

// GetFiles gets the filenames of the command's target and its dependencies.
func (mc *Cmd) GetFiles() (names []string) {
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

// HasChanged checks if the make command's target has changed since Progress()
// was last called. It is subtle, but Progress should be used during "grace
// mode" to find out when the make command has finished building itself
// and its dependencies. Afterwards, HasChanged should be used to check
// if the command should be restarted due to new changes.
func (mc *Cmd) HasChanged() bool {

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

// UpdateProgress checks how many targets need updating, and stores
// the result. It also updates the internal time to be used by HasChanged.
func (mc *Cmd) UpdateProgress() {
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
func (mc *Cmd) CheckProgress() (remaining int) {
	return mc.remaining
}

// String returns the underlying make command that gets run.
func (mc *Cmd) String() string {
	return mc.cmd.String()
}

// getDatabase runs the make query for this make command's
// target, and populates a new database with the results.
func (mc *Cmd) getDatabase() *makedb.Database {
	cmd := exec.Command("make", mc.queryArgs...)
	out, _ := cmd.Output()
	r := bytes.NewReader(out)
	db := makedb.NewDatabase()
	err := db.Populate(r)
	if err != nil {
		log.Fatalf("getDatabase for %s: %s", mc.queryArgs, err)
	}
	mc.db = &db
	return &db
}

// getRemaining counts how many targets need to be updated
// for this make command's target to be considered up to date.
func (mc *Cmd) getRemaining() (count int) {
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

// mustKill tries to kill the command and waits for it to finish.
// It will keep trying if there is a problem.
func (mc *Cmd) mustKill() {
	for {
		if err := mc.cmd.Kill(); err != nil {
			log.Printf(colors.Red("Remake: Error killing %s: %s"), mc, err)
			time.Sleep(1 * time.Second)
		} else {
			return
		}
	}
}