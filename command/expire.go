package command

import (
	"errors"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/history"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdExpire(c *cli.Context) error {
	outputLevel(c)

	// An empty filter matches everything, and this is the entry point to every
	// rotation.  A bare `locksmith expire` would mark every key in the
	// repository for removal from every system.
	if len(c.Args()) < 1 {
		output.Error("Must specify at least one filter; `expire` with no filter would expire every key")
		return errors.New("refusing to expire every key")
	}

	ml := lib.MainLibrary{Path: datadir(c)}

	log := history.Open(datadir(c), "expire")
	defer log.Close()

	return setPolicy(&ml, log, buildFilterFromContext(c), c.String("replace-with"))
}

func CmdUnexpire(c *cli.Context) error {
	outputLevel(c)

	if len(c.Args()) < 1 {
		output.Error("Must specify at least one filter; `unexpire` with no filter would unexpire every key")
		return errors.New("refusing to unexpire every key")
	}

	ml := lib.MainLibrary{Path: datadir(c)}

	log := history.Open(datadir(c), "unexpire")
	defer log.Close()

	return clearPolicy(&ml, log, buildFilterFromContext(c))
}

// setPolicy records what should happen to the matching keys.
//
// It writes a policy rather than setting a flag on the key record.  The key
// record is an observation, re-merged from reality on every fetch; intent is
// not, and storing intent there is what made expiry irreversible.
func setPolicy(ml *lib.MainLibrary, log *history.Log, filter Filter, replacement string) error {
	keys := ml.Keys()
	policies := ml.Policies()

	var replacementID data.ID
	if replacement != "" {
		found, err := findOneKey(keys, buildFilter([]string{replacement}))
		if err != nil {
			output.Error("Could not identify the replacement key:", err)
			return err
		}
		replacementID = found.Id()
	}

	matched := 0
	for k := range keys.ListMatching(keyFilter(filter)) {
		if replacementID != "" && k.Id() == replacementID {
			// Replacing a key with itself would have plan removing and adding
			// the same binding forever.
			output.Warn("Refusing to replace", k.Id(), "with itself")
			continue
		}

		policy := data.NewRemovePolicy(k.Id())
		if replacementID != "" {
			policy = data.NewReplacePolicy(k.Id(), replacementID)
		}

		if err := policies.Store(policy); err != nil {
			output.Error("Failed to record policy for", k.Id(), ":", err)
			return err
		}

		// Clear any flag written by an older locksmith, so one source of truth
		// remains once a key has been touched.  See effectivePolicy.
		if err := clearLegacyExpiry(keys, k); err != nil {
			output.Error("Failed to clear the legacy expiry flag:", err)
			return err
		}

		names := k.GetNames()
		log.Record(history.Event{
			Event: history.KeyExpired, Key: k.Id(), KeyName: names.Join(", "),
			Detail: policy.String(),
		})
		output.Verbose(policy)
		matched++
	}

	output.Normalf("Marked %d keys\n", matched)
	return nil
}

// clearPolicy revokes intent, which the old design could not do at any price:
// Expire only ever set the flag, Merge only ever ORed it on, and there was no
// command to undo it.
func clearPolicy(ml *lib.MainLibrary, log *history.Log, filter Filter) error {
	keys := ml.Keys()
	policies := ml.Policies()

	matched := 0
	for k := range keys.ListMatching(keyFilter(filter)) {
		had := false

		if _, err := policies.Fetch(k.Id()); err == nil {
			if err := policies.Delete(k.Id()); err != nil {
				output.Error("Failed to remove policy for", k.Id(), ":", err)
				return err
			}
			had = true
		}

		if k.IsDeprecated() {
			if err := clearLegacyExpiry(keys, k); err != nil {
				output.Error("Failed to clear the legacy expiry flag:", err)
				return err
			}
			had = true
		}

		if !had {
			continue
		}

		names := k.GetNames()
		log.Record(history.Event{
			Event: history.KeyUnexpired, Key: k.Id(), KeyName: names.Join(", "),
		})
		output.Verbose("no longer expiring", k.Id())
		matched++
	}

	output.Normalf("Unmarked %d keys\n", matched)
	return nil
}

// clearLegacyExpiry removes the Deprecated flag an older locksmith wrote onto
// the key record itself.
func clearLegacyExpiry(keys lib.KeyLibrary, k data.Key) error {
	if !k.IsDeprecated() {
		return nil
	}

	k.Unexpire()
	return keys.Store(k)
}
