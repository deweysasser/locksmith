package command

import (
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/keys"
	"github.com/deweysasser/locksmith/connection"
	"github.com/urfave/cli"
	"os"
	"fmt"
	"sync"
)

func CmdAdd(c *cli.Context) error {
	keylib := lib.NewKeylib(datadir())
	accounts := lib.NewAccountlib(datadir())

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

				rsystem := connection.NewSSHRemote(server)
				a := accounts.EnsureAccount(server)
				keys := rsystem.RetrieveKeys()
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
		keylib.Ingest(k)
	}

	fmt.Println(len(count), "keys found")

	return nil

}


