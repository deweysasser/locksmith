package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/keylib"
	// "golang.org/x/crypto/ssh"
	"os"
)

func CmdAdd(c *cli.Context) error {
	lib := keylib.KeyLib{datadir()}

	for _, a := range c.Args() {
		if info, _ := os.Stat(a); info != nil {
			lib.IngestFile(a)
		}
	}
	return nil

}
