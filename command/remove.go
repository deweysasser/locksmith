package command

import (
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/data"
)

func CmdRemove(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir(c)}

	filter := buildFilterFromContext(c)

	accounts := ml.Accounts()

	process(ml.Connections(), filter)
	process(accounts, filter)
	process(ml.Keys(), filter)
	processObject(ml.Changes(), filter, func(x interface{}) interface{} {
		if change, ok := x.(*data.Change); ok {
			if r, err := accounts.Fetch(string(change.Account)); err == nil {
				return r
			} else {
				return x
			}
		} else {
			return x
		}
	})

	return nil
}

type Stringer func(interface{}) interface{}

func processObject(l lib.Library, filter Filter, stringer Stringer) {
	for obj := range l.List() {
		o := stringer(obj)
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


func process(l lib.Library, filter Filter) {
	var s Stringer = func (x interface{}) interface{} { return x }
	processObject(l, filter, s)
}

