package command

import (
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

func CmdDisplayLib(c *cli.Context) error {
	output.Level = output.DebugLevel

	ml := lib.MainLibrary{Path: datadir(c)}

	output.Debug("Loading files")
	ml.Keys().Load()
	output.Debug("Cache")
	ml.Keys().PrintCache()
	return nil
}
