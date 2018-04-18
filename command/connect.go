package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/connection"
)

func CmdConnect(c *cli.Context) error {
	ml := lib.MainLibrary{Path:datadir()}
	clib := ml.Connections()

	for _, a := range c.Args() {

		conn := connection.Create(a)
		clib.Store(conn)
	}

	return nil
}

