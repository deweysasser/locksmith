package command

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdAdd(c *cli.Context) error {
	outputLevel(c)

	// This stuff should be in the arg parser
	if c.String("key") == "" {
		output.Error("Must specify -key")
		return nil
	}

	if len(c.Args()) < 1 {
		output.Error("Must specify at least one filter for accounts")
		return nil
	}

	ml := lib.MainLibrary{Path: datadir(c)}
	changes := ml.Changes()

	skeyFilter := []string{c.String("key")}
	output.Debug("key filter is", skeyFilter)
	keys := getKeyIds(ml.Keys(), ml.Policies(), keyFilter(buildFilter(skeyFilter)))

	output.Debug("Keys to add:", keys)

	for account := range ml.Accounts().ListMatching(accountFilter(buildFilterFromContext(c))) {
		var bindings []data.KeyBindingImpl
		for _, k := range keys {
			output.Verbose("Adding", k, "to account", account)

			bindings = append(bindings, data.KeyBindingImpl{
				KeyID:    k,
				Location: data.AUTHORIZED_KEYS,
			})
		}

		changes.Store(data.Change{
			Manual:  true,
			Type:    "Change",
			Account: account.Id(),
			Add:     bindings,
			Remove:  make([]data.KeyBindingImpl, 0),
		})
	}

	return nil
}

func getKeyIds(library lib.KeyLibrary, policies lib.PolicyLibrary, predicate lib.KeyPredicate) []data.ID {
	keys := make([]data.ID, 0)

	for k := range library.ListMatching(predicate) {
		output.Debug("Checking key", k)
		// Never hand out a key that is on its way off systems: binding it
		// somewhere new would have the next plan immediately remove it again.
		if _, doomed := effectivePolicy(policies, k); !doomed {
			keys = append(keys, k.Id())
		}
	}

	return keys
}
