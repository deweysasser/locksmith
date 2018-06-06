package command

import (
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/config"
	"os"
)

func CmdRemove(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: config.Property.LOCKSMITH_REPO}

	filter := buildFilterFromContext(c)

	if len(c.Args()) < 1{
		output.Error("Remove cannot be called without some filter")
		os.Exit(1)
	}

	accounts := ml.Accounts()
	connections := ml.Connections()
	keys := ml.Keys()
	changes := ml.Changes()

	// Why golang, why???  DRY!!!
	for conn := range connections.ListMatching(func(connection connection.Connection) bool { return filter(connection) }) {
		output.Verbose("Deleting", conn)
		connections.DeleteObject(conn)
	}

	for account := range accounts.ListMatching(func(account data.Account) bool { return filter(account) }) {
		output.Verbose("Deleting", account)
		accounts.DeleteObject(account)
	}

	for key := range keys.ListMatching(func(key data.Key) bool { return filter(key) }) {
		output.Verbose("Deleting", key)
		keys.DeleteObject(key)
	}

	for change := range changes.ListMatching(func(change data.Change) bool { return filter(changestr(accounts, change)) }) {
		output.Verbose("Deleting", change)
		changes.DeleteObject(change)
	}

	return nil
}

func changestr(accounts lib.AccountLibrary, change data.Change) interface{} {
	if r, err := accounts.Fetch(change.Account); err == nil {
		return r
	} else {
		return change
	}
}
