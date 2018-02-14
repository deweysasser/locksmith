package command

import "github.com/urfave/cli"
import "github.com/deweysasser/locksmith/keylib"

//import "golang.org/x/crypto/ssh"
import "fmt"
import "os"

func CmdAdd(c *cli.Context) error {
	lib := keylib.KeyLib{"lsdata"}

	for _, a := range c.Args() {
		if info, _ := os.Stat(a); info != nil {
			fmt.Printf("path is %s\n", a)
			lib.IngestFile(a)
		}
	}
	return nil

}
