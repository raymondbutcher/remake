package makecmd

import (
	"bytes"
	"log"
	"os/exec"
	"time"

	"github.com/raymondbutcher/remake/colors"
	"github.com/raymondbutcher/remake/makedb"
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
// was last called. It is subtle, but UpdateProgress should be used during
// "grace mode" to find out when the make command has finished building itself
// and its dependencies. Afterwards, HasChanged should be used to check
// if the command should be restarted due to new changes.
func (mc *Cmd) HasChanged() bool {

	if mc.progressed.IsZero() {
		panic("Cannot use HasChanged before UpdateProgress")
	}
	if !mc.usedChanged {
		mc.usedChanged = true
	}

	return mc.getRemaining() > 0
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

// getRemaining returns the number of targets that need to be updated
// for this make command's target to be considered up to date.
func (mc *Cmd) getRemaining() (count int) {
	return mc.getDatabase().GetPendingTargets(mc.Target, mc.progressed)
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
