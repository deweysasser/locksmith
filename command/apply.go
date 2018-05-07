package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/deweysasser/locksmith/data"
)

func CmdApply(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir(c)}
	filter := buildFilterFromContext(c)

	for ch := range ml.Changes().List() {
		if change, ok := ch.(*data.Change); ok {
			aid := change.Account
			if acct , err := ml.Accounts().Fetch(string(aid)); err == nil {
				if (filter(acct)) {
					output.Debug("Applying changes for ", acct)
					if account, ok := acct.(data.Account); ok {
						cid := account.ConnectionID()
						if conn, err := ml.Connections().Fetch(string(cid)); err == nil {
							// Finally, the main event
							output.Normal("via", conn)
						} else {
							output.Error(cid, "is not a connection")
						}
						output.Normal("Connection", cid)
					} else {
						output.Error(acct, "is not an account")
					}
				}
			} else {
				output.Error("Failed to find account for", aid)
			}
		} else {
			output.Error("Change is not a type change", ch)
		}
	}

	return nil
}
