package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdPlan(c *cli.Context) error {
	outputLevel(c)

	filter := buildFilterFromContext(c)

	ml := lib.MainLibrary{Path: datadir(c)}

	output.Debug("Calculating changes")
	calculateChanges(ml.Accounts(), ml.Keys(), ml.Changes(), ml.Policies(), filter)

	output.Debug("Showing changes")
	showPendingChanges(ml.Changes(), ml.Keys(), ml.Accounts(), AcceptAll)
	return nil
}

func showPendingChanges(changelib lib.ChangeLibrary, keylib lib.KeyLibrary, accountlib lib.AccountLibrary, filter Filter) {
	output.Debug("showing pending changes")
	for change := range changelib.List() {
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

func printChange(keylib lib.KeyLibrary, add data.KeyBindingImpl, s string) {
	if key, err := keylib.Fetch(add.KeyID); err == nil {
		output.Verbose("  ", s, key)
	} else {
		output.Error("Cannot find key", add, "in change")
	}
}

// effectivePolicy is what should happen to a key, taking the policy library as
// authoritative and falling back to the flag an older locksmith wrote onto the
// key record itself.
//
// The fallback exists so an existing repository keeps working without a
// migration step.  Once `expire` or `unexpire` touches a key the flag is
// cleared, so a repository converges on policy-only as it is used.
func effectivePolicy(policies lib.PolicyLibrary, key data.Key) (data.KeyPolicy, bool) {
	if p, err := policies.Fetch(key.Id()); err == nil {
		return p, true
	}

	if repl := key.ReplacementID(); repl != "" {
		return data.NewReplacePolicy(key.Id(), repl), true
	}
	if key.IsDeprecated() {
		return data.NewRemovePolicy(key.Id()), true
	}

	return data.KeyPolicy{}, false
}

// calculateChanges derives the pending work from policy and inventory.
//
// It is a diff, not an accumulator.  Every change it owns is recomputed from
// scratch, and a change that is no longer warranted is deleted rather than left
// behind -- previously `plan` only ever wrote, so a change survived the
// condition that produced it and `apply` kept reapplying it.
//
// Changes created by `add` are left alone: those are a direct instruction from
// the operator, not something plan derives, and overwriting them silently
// discarded work the user had just asked for.
func calculateChanges(accountLib lib.AccountLibrary, keylib lib.KeyLibrary, changelib lib.ChangeLibrary, policies lib.PolicyLibrary, filter Filter) {
	for account := range accountLib.ListMatching(accountFilter(filter)) {
		output.Debug("Working on account", account)

		var additions []data.KeyBindingImpl
		var removals []data.KeyBindingImpl

		for binding := range account.Bindings() {
			output.Debug("Examining binding", binding)

			key, err := keylib.Fetch(binding.KeyID)
			if err != nil {
				// A binding can outlive the key it names, for instance after
				// `remove`.  Nothing to decide about a key we do not have.
				output.Debug("No key for binding", binding.KeyID, ":", err)
				continue
			}

			policy, found := effectivePolicy(policies, key)
			if !found {
				continue
			}

			if policy.Removes() {
				removals = append(removals, binding)
			}
			if policy.Disposition == data.DispositionReplace && policy.Replacement != "" {
				additions = append(additions, newBinding(binding, policy.Replacement))
			}
		}

		storeOrClearChange(changelib, account.Id(), additions, removals)
	}
}

// storeOrClearChange writes the derived change for an account, or removes the
// stored one when there is no longer anything to do.
func storeOrClearChange(changelib lib.ChangeLibrary, account data.ID, additions, removals []data.KeyBindingImpl) {
	existing, err := changelib.Fetch(account)
	hasExisting := err == nil

	if hasExisting && existing.Manual {
		// `add` wrote this.  Plan does not own it and must not overwrite it.
		output.Debug("Leaving the manually added change for", account, "alone")
		return
	}

	if len(additions) == 0 && len(removals) == 0 {
		if hasExisting {
			output.Verbose("No longer any change needed for", account)
			if e := changelib.Delete(account); e != nil {
				output.Error("Failed to remove the stale change for", account, ":", e)
			}
		}
		return
	}

	if e := changelib.Store(data.Change{
		Type:    "Change",
		Account: account,
		Add:     additions,
		Remove:  removals,
	}); e != nil {
		output.Error("Failed to store the change for", account, ":", e)
	}
}

func newBinding(binding data.KeyBindingImpl, key data.ID) data.KeyBindingImpl {
	binding.KeyID = key
	return binding
}
