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
