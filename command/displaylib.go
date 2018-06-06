package command

import (
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/config"
)

func CmdDisplayLib(c *cli.Context) error {
	output.Level = output.DebugLevel

	ml := lib.MainLibrary{Path: config.Property.LOCKSMITH_REPO}

	output.Debug("Cache")
	ml.Keys().PrintCache()
	return nil
}
