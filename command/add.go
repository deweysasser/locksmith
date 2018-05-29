package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
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
	keys := getKeyIds(ml.Keys(), keyFilter(buildFilter(skeyFilter)))

	output.Debug("Keys to add:", keys)


	for account := range ml.Accounts().ListMatching(accountFilter(buildFilterFromContext(c))) {
		var bindings []data.KeyBinding
		for _, k := range keys {
			output.Verbose("Adding", k, "to account", account)

			bindings = append(bindings, data.KeyBinding{
				k,
				data.AUTHORIZED_KEYS,
				"",
			})
		}

		changes.Store(data.Change{"Change", account.Id(), bindings, make([]data.KeyBinding, 0)})
	}

	return nil
}

func getKeyIds(library lib.KeyLibrary, predicate lib.KeyPredicate) []data.ID {
	keys := make([]data.ID, 0)

	for k := range library.ListMatching(predicate) {
		output.Debug("Checking key", k)
		if !k.IsDeprecated() {
			keys = append(keys, k.Id())
		}
	}

	return keys
}
