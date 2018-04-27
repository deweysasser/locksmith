package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/output"
	"github.com/deweysasser/locksmith/lib"
)

func CmdDisplayLib(c *cli.Context) error {
	output.Level = output.DebugLevel

	ml := lib.MainLibrary{Path: datadir()}

	output.Debug("Loading files")
	ml.Keys().Load()
	output.Debug("Cache")
	ml.Keys().PrintCache()
	return nil
}

