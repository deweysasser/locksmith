package command

import (
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/connection"
	"github.com/urfave/cli"
//	"os"
//	"fmt"
	"sync"
)

func CmdAdd(c *cli.Context) error {
	library := lib.NewLibrary(datadir())
	keylib := library.Keylib()
	accounts := library.Accountlib()

	wg := sync.WaitGroup{}

	for _, a := range c.Args() {
		wg.Add(1)
		c := connection.Create(a)
		go func(c connection.Connection) {
			c.Fetch(accounts, keylib)
			wg.Done()
		}(c)
	}

	wg.Wait()
	library.Save()

	return nil

}


