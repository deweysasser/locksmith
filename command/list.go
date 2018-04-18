package command

import (
	"github.com/urfave/cli"
	"fmt"
	"github.com/deweysasser/locksmith/lib"
)

func CmdList(c *cli.Context) error {
	ml := lib.MainLibrary{Path: datadir()}


	filter := buildFilter(c.Args())

	for i:= range ml.Connections().List() {
		s := fmt.Sprintf("connection %s", i)
		if filter(s) {
			fmt.Println(s)
		}
	}

	return nil
}
