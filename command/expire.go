package command


import (
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/data"
)

func CmdExpire(c *cli.Context) error {
	output.Level = output.DebugLevel
	ml := lib.MainLibrary{Path: datadir(c)}

	filter := buildFilterFromContext(c)

	keys := make(chan data.Key)

	library := ml.Keys()
	go func() {
		for i := range library.List() {
			if filter(keyString(i, "")) {
				if k, ok := i.(data.Key) ; ok {
					k.Expire()
					keys <- k
				}  else {
					output.Error(i, "is not a key")
				}
			}
		}
		close(keys)
	}()

	for k:= range keys {
		library.Store(k)
	}

	return nil
}

