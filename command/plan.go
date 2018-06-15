package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/config"
)

func CmdPlan(c *cli.Context) error {
	outputLevel(c)

	filter := buildFilterFromContext(c)

	ml := lib.MainLibrary{Path: config.Property.LOCKSMITH_REPO}

	output.Debug("Calculating changes")
	calculateChangesForAllAccounts(ml.Accounts(), ml.Keys(), ml.Changes(), filter)

	output.Debug("Showing changes")
	showPendingChanges(ml.Changes().List(), ml.Keys(), ml.Accounts(), AcceptAll)
	return nil
}

func showPendingChanges(changes <- chan data.Change, keylib lib.KeyLibrary, accountlib lib.AccountLibrary, filter Filter) {
	output.Debug("showing pending changes")
	for change := range changes {
		output.Debug("Change is", change)
		if acct, err := accountlib.Fetch(change.Account); err == nil {
			s := fmt.Sprint("change ", acct)
			output.Debug("Checking change", s)
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
	}
}

/** Stringify a change
 */

 func changeString(change data.Change, accountlib lib.AccountLibrary) (string, error){
	 if acct, err := accountlib.Fetch(change.Account); err == nil {
		 return fmt.Sprint("change ", acct), nil
	 } else {
	 	return fmt.Sprintf("(error looking up account %s)", change.Account), err
	 }
 }

func printChange(keylib lib.KeyLibrary, add data.KeyBindingImpl, s string) {
	output.Verbose(bindingString(add, keylib, s, "  "))
}

/** Get a string describing a binding
 */
func bindingString(binding data.KeyBindingImpl, keylib lib.KeyLibrary, changeType string, prefix string) string {
	if key, err := keylib.Fetch(binding.KeyID); err == nil {
		return fmt.Sprintf("%s%s %s", prefix, changeType, key)
	} else {
		output.Error("Cannot find key", changeType, "in change")
		return fmt.Sprintf("(error finding key %s)", binding.KeyID)
	}
}

func calculateChangesForAllAccounts(accountLib lib.AccountLibrary, keylib lib.KeyLibrary, changelib lib.ChangeLibrary, filter Filter) {
	for a := range accountLib.List() {
		if account, ok := a.(data.Account); ok {
			if !filter(account) {
				continue
			}
			output.Debug("Working on account", account)
			calculateAccountChanges(account, keylib, changelib)
		} else {
			output.Error("Account list was not an account")
		}
	}
}

func calculateAccountChanges(account data.Account, keylib lib.KeyLibrary, changelib lib.ChangeLibrary) (change *data.Change, count int) {
	var additions []data.KeyBindingImpl
	var removals []data.KeyBindingImpl

	for binding := range account.Bindings() {
		output.Debug("Examining binding", binding)
		if bk, err := keylib.Fetch(binding.KeyID); err == nil {
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
			output.Error(fmt.Sprintf("Failed to lookup key '%s' in account '%s': %s", binding.KeyID, account.Id(), err))
		}
	}
	if len(additions) > 0 || len(removals) > 0 {
		change := data.Change{"Change", account.Id(), additions, removals}
		changelib.Store(change)
		return &change, len(additions) + len(removals)
	} else {
		return nil, 0
	}
}

func newBinding(binding data.KeyBindingImpl, key data.ID) data.KeyBindingImpl {
	binding.KeyID = key
	return binding
}
