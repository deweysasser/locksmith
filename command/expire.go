package command

import (
	"errors"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdExpire(c *cli.Context) error {
	outputLevel(c)

	// An empty filter matches everything, and expiry is one-way: there is no
	// unexpire, and Merge only ever ORs the flag on.  A bare `locksmith expire`
	// would irreversibly deprecate every key in the repository.
	if len(c.Args()) < 1 {
		output.Error("Must specify at least one filter; `expire` with no filter would expire every key")
		return errors.New("refusing to expire every key")
	}

	ml := lib.MainLibrary{Path: datadir(c)}

	filter := buildFilterFromContext(c)

	keys := make(chan data.Key)

	library := ml.Keys()
	go func() {
		for i := range library.List() {
			if filter(keyString(i, "")) {
				if k, ok := i.(data.Key); ok {
					k.Expire()
					keys <- k
				} else {
					output.Error(i, "is not a key")
				}
			}
		}
		close(keys)
	}()

	for k := range keys {
		library.Store(k)
	}

	return nil
}
