package makecmd

// MonitorMode monitors the make command's target to see if it needs updating.
// If it does, and the command is still running, then it will kill the command.
// It will not return until it needs updating and it is not running.
func (cmd *Cmd) MonitorMode(checkChannel <-chan struct{}) {
	for {
		select {
		case <-cmd.cmd.Finished():
			// The command exited. Don't actually do anything because
			// this doesn't mean that the make target needs updating.
		case <-checkChannel:
			if cmd.HasChanged() {
				// The make target is no longer up to date. Kill the process
				// if it is still running, and then return so the make command
				// can be started again.
				cmd.mustKill()
				return
			}
		}
	}
}
