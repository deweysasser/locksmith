package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdList(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir(c)}

	keyToAccounts := make(map[data.ID][]data.ID)

	// Build the map if we're going to need it
	if output.IsLevel(output.VerboseLevel) {
		for a := range ml.Accounts().List() {
			account := a.(data.Account)
			for _, b := range account.Bindings() {
				keyToAccounts[b.KeyID] = append(keyToAccounts[b.KeyID], account.Id())
			}
		}
	}

	filter := buildFilterFromContext(c)

	printConnections(ml.Connections(), filter)
	printAccounts(ml.Accounts(),filter, ml)
	printKeys(ml.Keys(), ml.Accounts(), keyToAccounts, filter)

	return nil
}

func printKeys(keys lib.Library, accounts lib.Library, keyToAccounts map[data.ID][]data.ID, filter Filter) {
	for i := range keys.List() {
		s := keyString(i, "")
		if filter(s) {
			output.Normal(s)
			if output.IsLevel(output.VerboseLevel) {
				accts := keyToAccounts[i.(data.Ider).Id()]
				for _, a := range accts {
					if acct, err := accounts.Fetch(string(a)); err == nil {
						output.Verbose(accountString(acct, "  "))
					} else {
						output.Debug("Unable to fetch account ID", a)
					}
				}
			}
			if output.IsLevel(output.DebugLevel) {
				for _, id := range i.(data.Key).Identifiers() {
					output.Debug("  ", "ID", id)
				}
			}
		}
	}
}

func printAccounts(accounts lib.Library, filter Filter, ml lib.MainLibrary) {
	for i := range accounts.List() {
		s := accountString(i, "")
		if filter(s) {
			output.Normal(s)
			if output.IsLevel(output.VerboseLevel) {
				outputKeysFor(i.(data.Account), ml.Keys())
			}
		}
	}
}

func printConnections(connections lib.Library, filter Filter) {
	for i := range connections.List() {

		s := connectionString(i, "")
		if filter(s) {
			output.Normal(s)
			if output.IsLevel(output.VerboseLevel) {
			}
		}
	}
}

func connectionString(i interface{}, prefix string) string {
	return fmt.Sprintf("%sconnection %s", prefix, i)
}

func accountString(i interface{}, prefix string) string {
	return fmt.Sprintf("%saccount %s", prefix, i)
}

func keyString(i interface{}, prefix string) string {
	return fmt.Sprintf("%skey %s", prefix, i)
}

func outputKeysFor(a data.Account, keys lib.Library) {
	for _, k := range a.Bindings() {
		s, _:= k.Describe(keys)
		output.Verbose("  ", s)
	}
}
