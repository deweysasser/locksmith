package command

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"reflect"
	"fmt"
)

func CmdPlan(c *cli.Context) error {
	outputLevel(c)

	filter := buildFilterFromContext(c)

	ml := lib.MainLibrary{Path: datadir(c)}

	calculateChanges(ml.Accounts(), ml.Keys(), ml.Changes(), filter)

	showPendingChanges(ml.Changes(), ml.Keys(), ml.Accounts(), AcceptAll)
	return nil
}

func showPendingChanges(changelib lib.ChangeLibrary, keylib lib.Library, accountlib lib.Library, filter Filter) {
	for ch := range changelib.List() {
		if change, ok := ch.(*data.Change); ok {
			if acct, err := accountlib.Fetch(string(change.Account)); err == nil {
				s := fmt.Sprint("change ", acct)
				if filter(s) {
					output.Normal(s)
					if output.IsLevel(output.VerboseLevel) {
						for _, add := range change.Add {
							printChange(keylib, add, "add")
						}
						for _, remove := range change.Remove {
							printChange(keylib, remove, "remove")
						}
					}
				}
			} else {
				output.Error("Could not find account", change.Account)
			}
		} else {
			output.Error("Change is not a change: ", ch, reflect.TypeOf(ch))
		}

	}
}

func printChange(keylib lib.Library, add data.KeyBinding, s string) {
	if key, err := keylib.Fetch(string(add.KeyID)); err == nil {
		output.Verbose("  ", s, key)
	} else {
		output.Error("Cannot find key", add, "in change")
	}
}

func calculateChanges(accountLib lib.Library, keylib lib.Library, changelib lib.ChangeLibrary, filter Filter) {
	for a := range accountLib.List() {
		if account, ok := a.(data.Account); ok {
			if !filter(account) {
				continue
			}
			output.Debug("Working on account", account)
			var additions []data.KeyBinding
			var removals []data.KeyBinding

			for _, binding := range account.Bindings() {
				output.Debug("Examining binding", binding)
				if bk, err := keylib.Fetch(string(binding.KeyID)); err == nil {
					if key, ok := bk.(data.Key); ok {
						if key.IsDeprecated() {
							removals = append(removals, binding)
						}
						if repl := key.ReplacementID(); repl != "" {
							additions = append(additions, newBinding(binding, repl))
						}
					} else {
						output.Error("Discovered key which is not a key", bk)
					}
				} else {
					output.Error("Failed to lookup key", binding.KeyID, err)
				}
			}
			if len(additions) > 0 || len(removals) > 0 {
				changelib.Store(data.Change{"Change", account.Id(), additions, removals})
			}
		} else {
			output.Error("Account list was not an account")
		}

	}
}

func newBinding(binding data.KeyBinding, key data.ID) data.KeyBinding {
	binding.KeyID = key
	return binding
}
