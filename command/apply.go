package command

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/connection"
)

func CmdApply(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir(c)}
	filter := buildFilterFromContext(c)

	for change := range ml.Changes().List() {
		aid := change.Account
		if acct, err := ml.Accounts().Fetch(aid); err == nil {
			if filter(acct) {
				output.Debug("Applying changes for ", acct)
				if account, ok := acct.(data.Account); ok {
					cid := account.ConnectionID()
					if conn, err := ml.Connections().Fetch(cid); err == nil {
						if changer, ok := conn.(connection.Changer); ok {
							// Finally, the main event
							output.Normal("via", changer)
						} else {
							output.Warn("Connection", conn, "cannot change keys")
						}
					} else {
						output.Error(cid, "is not a connection")
					}
				} else {
					output.Error(acct, "is not an account")
				}
			}
		} else {
			output.Error("Failed to find account for", aid)
		}
	}

	return nil
}
