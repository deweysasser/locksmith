package command

import (
	"github.com/deweysasser/locksmith/keylib"
	"github.com/deweysasser/locksmith/accountlib"
	"github.com/deweysasser/locksmith/keys"
	"github.com/deweysasser/locksmith/remote"
	"github.com/urfave/cli"
	"os"
	"fmt"
	"sync"
)

func CmdAdd(c *cli.Context) error {
	lib := keylib.New(datadir())
	accounts := accountlib.New(datadir())

	kchan := make(chan keys.Key)



	wg := sync.WaitGroup{}

	for _, a := range c.Args() {
		wg.Add(1)
		if info, _ := os.Stat(a); info != nil {
			go func(path string) {
				fmt.Println("Reading", path)
				kchan <- keys.Read(path)
				wg.Done()
			}(a)
		} else  {
			go func (server string) {
				fmt.Printf("Retrieving from %s\n", server)

				a := accounts.EnsureAccount(server)
				keys := remote.RetrieveKeys(server)
				a.SetKeys(keys)
				for _, k:= range(keys) {
					kchan <- k
				}
				wg.Done()
			}(a)
		}
	}

	go func() {
		wg.Wait()
		close(kchan)
	}()

	count := make(map[keys.KeyID]keys.Key)
	
	// Ingest the keys
	for k := range(kchan) {
		count[k.Id()]=k
		lib.Ingest(k)
	}

	fmt.Println(len(count), "keys found")

	return nil

}


