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
// It looks under *every* identifier the key answers to, not just the primary
// one.  A key's primary ID changes when fingerprint-only material (from AWS or
// Digital Ocean) is later upgraded by a fetch that learns the real public key,
// and a policy stored under the old ID would otherwise be silently orphaned --
// the repository would say the key was expired while the fleet kept honouring
// it.
func effectivePolicy(policies lib.PolicyLibrary, key data.Key) (data.KeyPolicy, bool) {
	for _, id := range key.Identifiers() {
		if p, err := policies.Fetch(id); err == nil {
			return p, true
		}
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
// Bindings `add` requested are carried forward untouched: those are a direct
// instruction from the operator, not something plan derives, and overwriting
// them silently discarded work the user had just asked for.  Plan still owns
// the derived half of the same change -- bailing out of the whole account
// because an `add` was pending meant a policy-driven *removal* was quietly
// dropped, which is the one thing this tool must never do.
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

	manual := hasExisting && existing.Manual
	var manualAdd []data.KeyBindingImpl
	if manual {
		manualAdd = existing.ManualAdd
		if len(manualAdd) == 0 {
			// Written by a locksmith that kept both kinds in Add.
			manualAdd = existing.Add
		}
	}

	if len(additions) == 0 && len(removals) == 0 && len(manualAdd) == 0 {
		if hasExisting {
			output.Verbose("No longer any change needed for", account)
			if e := changelib.Delete(account); e != nil {
				output.Error("Failed to remove the stale change for", account, ":", e)
			}
		}
		return
	}

	if e := changelib.Store(data.Change{
		Type:      "Change",
		Account:   account,
		Manual:    manual,
		ManualAdd: manualAdd,
		Add:       additions,
		Remove:    removals,
	}); e != nil {
		output.Error("Failed to store the change for", account, ":", e)
	}
}

func newBinding(binding data.KeyBindingImpl, key data.ID) data.KeyBindingImpl {
	binding.KeyID = key
	return binding
}
