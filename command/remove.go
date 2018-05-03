package command

import (
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdRemove(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir(c)}

	filter := buildFilterFromContext(c)

	process(ml.Connections(), filter)
	process(ml.Accounts(), filter)
	process(ml.Keys(), filter)

	return nil
}

func process(l lib.Library, filter Filter) {
	for o := range l.List() {
		if filter(o) {
			output.Verbose("Removing ", o)
			if e := l.DeleteObject(o); e == nil {
				output.Debug(o, "removed")
			} else {
				output.Errorf("Failed to delete '%s' with id '%s': %s", o, l.Id(o), e)
			}
		}
	}
}
