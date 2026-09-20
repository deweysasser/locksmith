package command

import (
	"errors"
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdRemove(c *cli.Context) error {

	outputLevel(c)

	// An empty filter matches everything, so a bare `locksmith remove` would
	// delete every connection, account, key and change in the repository.
	if len(c.Args()) < 1 {
		output.Error("Must specify at least one filter; `remove` with no filter would remove everything")
		return errors.New("refusing to remove every object")
	}

	ml := lib.MainLibrary{Path: datadir(c)}

	filter := buildFilterFromContext(c)

	accounts := ml.Accounts()
	connections := ml.Connections()
	keys := ml.Keys()
	changes := ml.Changes()
	policies := ml.Policies()

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

	// Policies are keyed by key ID, so once the key is gone nothing can name
	// the policy again -- not `list`, not `unexpire`, both of which walk the
	// key library.  Left behind, it silently re-expires the key if it is ever
	// re-fetched.
	for policy := range policies.ListMatching(func(p data.KeyPolicy) bool { return filter(p) }) {
		output.Verbose("Deleting", policy)
		policies.DeleteObject(policy)
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
