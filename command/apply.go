package command

import (
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/connection"
)

func CmdApply(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir(c)}
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
							if account, err := accounts.Fetch(change.Account) ; err == nil {
								if err := changer.Update(account, change.Add, change.Remove, keys); err != nil {
									output.Error("Failed to add keys:", err)
									continue
								}
							} else {
								output.Error("Canot lookup account", change.Account)
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
