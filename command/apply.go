package command

import (
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/history"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdApply(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir(c)}

	if c.Bool("dry-run") {
		return dryRunApply(&ml, buildFilterFromContext(c))
	}

	// The most important entries in the history: everything else records what
	// locksmith learned, these record what it changed on someone else's
	// machine.
	log := history.Open(datadir(c), "apply")
	defer log.Close()
	filter := buildFilterFromContext(c)
	accounts := ml.Accounts()
	keys := ml.Keys()

	for change := range ml.Changes().List() {
		if acct, err := ml.Accounts().Fetch(change.Account); err == nil {
			if filter(acct) {
				output.Debug("Applying changes for ", acct)
				cid := acct.ConnectionID()
				if conn, err := ml.Connections().Fetch(cid); err == nil {
					if changer, ok := conn.(connection.Changer); ok {
						// Finally, the main event
						output.Debug("via", changer)
						if account, err := accounts.Fetch(change.Account); err == nil {
							if err := changer.Update(account, change.Additions(), change.Remove, keys); err != nil {
								output.Error("Failed to add keys:", err)
								continue
							} else {
								recordApplied(log, change)
								ml.Changes().DeleteObject(change)
							}
						} else {
							output.Error("Cannot lookup account", change.Account)
						}
					} else {
						output.Warn("Connection", conn, "cannot change keys")
					}
				} else {
					output.Error(cid, "is not a connection")
				}
			}
		} else {
			output.Error("Failed to find account for", change.Account)
		}
	}

	return nil
}

// dryRunApply prints the commands a real apply would run, and does nothing
// else: it opens no connection, changes no remote system, writes no history and
// deletes no change.
//
// It reads the stored changes rather than re-deriving them, so what it shows is
// the artifact `apply` will consume, not a fresh computation that could differ.
// And it renders each command through the same builders the real path executes,
// so the preview cannot drift away from the behaviour it claims to describe.
func dryRunApply(ml *lib.MainLibrary, filter Filter) error {
	changes := 0
	commands := 0

	for change := range ml.Changes().List() {
		acct, err := ml.Accounts().Fetch(change.Account)
		if err != nil {
			output.Error("Failed to find account for", change.Account)
			continue
		}
		if !filter(acct) {
			continue
		}

		changes++

		conn, err := ml.Connections().Fetch(acct.ConnectionID())
		if err != nil {
			output.Error(acct.ConnectionID(), "is not a connection")
			continue
		}

		output.Normal("would change", acct)

		previewer, ok := conn.(connection.Previewer)
		if !ok {
			// Distinguish "cannot be changed at all" from "can be changed but
			// cannot describe itself", so a silent dry run never reads as
			// "nothing would happen".
			if _, changeable := conn.(connection.Changer); changeable {
				output.Warn("  connection", conn, "cannot preview its changes; run without --dry-run to apply")
			} else {
				output.Warn("  connection", conn, "cannot change keys; apply would skip it")
			}
			continue
		}

		lines, err := previewer.Preview(acct, change.Additions(), change.Remove, ml.Keys())
		if err != nil {
			// The real apply would fail here too, and for the same reason --
			// which is exactly what a dry run is for.
			output.Error("  cannot build the commands for", acct, ":", err)
			continue
		}

		if len(lines) == 0 {
			output.Normal("  (nothing to do)")
			continue
		}

		for _, line := range lines {
			output.Normal("   ", line)
			commands++
		}
	}

	output.Normalf("dry run: %d change(s), %d command(s); nothing was executed\n", changes, commands)
	return nil
}

// recordApplied writes one event per binding actually changed on the remote
// host.  It runs only after Update reported success, so the log records work
// done rather than work attempted.
func recordApplied(log *history.Log, change data.Change) {
	for _, b := range change.Additions() {
		log.Record(history.Event{
			Event: history.AppliedAdd, Key: b.KeyID,
			Account: change.Account, Location: b.Location,
		})
	}
	for _, b := range change.Remove {
		log.Record(history.Event{
			Event: history.AppliedRemove, Key: b.KeyID,
			Account: change.Account, Location: b.Location,
		})
	}
}
