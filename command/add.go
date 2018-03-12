package command

import (
	"github.com/deweysasser/locksmith/keylib"
	"github.com/deweysasser/locksmith/keys"
	"github.com/deweysasser/locksmith/remote"
	"github.com/urfave/cli"
	"os"
	"sync"
)

func CmdAdd(c *cli.Context) error {
	lib := keylib.New(datadir())

	kchan := make(chan keys.Key)



	wg := sync.WaitGroup{}

	for _, a := range c.Args() {
		wg.Add(1)
		if info, _ := os.Stat(a); info != nil {
			go func() {
				kchan <- keys.Read(a)
				wg.Done()
			}()
		} else  {
			go func (server string) {
				remote.RetrieveKeys(server, kchan)
				wg.Done()
			}(a)
		}
	}

	go func() {
		wg.Wait()
		close(kchan)
	}()

	// Ingest the keys
	for k := range(kchan) {
		lib.Ingest(k)
	}

	return nil

}


