package command

import (
	"github.com/deweysasser/locksmith/keylib"
	"github.com/deweysasser/locksmith/keys"
	"github.com/deweysasser/locksmith/remote"
	"github.com/urfave/cli"
	"fmt"
	"os"
	"sync"
)

func CmdAdd(c *cli.Context) error {
	lib := keylib.KeyLib{datadir()}

	kchan := make(chan keys.Key)



	wg := sync.WaitGroup{}

	for _, a := range c.Args() {
		if info, _ := os.Stat(a); info != nil {
			kchan <- keys.Read(a)
		} else  {
			fmt.Println("+1")
			wg.Add(1)
			go func () {
				remote.RetrieveKeys(a, kchan)
				fmt.Println("Done retrieving keys")
				fmt.Println("-1")
				wg.Done()
			}()
		}
	}

	go func() {
		fmt.Println("Waiting")
		wg.Wait()
		close(kchan)
	}()

	// Ingest the keys
	for k := range(kchan) {
		fmt.Println("Ingesting key", k)
		lib.Ingest(k)
	}

	return nil

}


