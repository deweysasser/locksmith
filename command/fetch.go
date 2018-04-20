package command

import (
"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"sync"
	"fmt"
)

func CmdFetch(c *cli.Context) error {
	libWG := sync.WaitGroup{}
	ml := lib.MainLibrary{Path: datadir()}

	fKeys := data.NewFanInKey()
	fAccounts := data.NewFanInAccount()

	libWG.Add(1)
	go ingestKeys(ml.Keys(), fKeys.Output(), &libWG)


	libWG.Add(1)
	go ingestAccounts(ml.Accounts(), fAccounts.Output(), &libWG)

	filter := buildFilter(c.Args())

	for conn := range ml.Connections().List() {
		if filter(conn) {
			go fetchFrom(conn, fKeys.Input(), fAccounts.Input())
		}
	}

	fKeys.Wait()
	fAccounts.Wait()
	libWG.Wait()
	return nil
}

func fetchFrom(conn interface{}, keys chan data.Key, accounts chan data.Account) {
	switch conn.(type) {
	case connection.Connection:
			//fmt.Println("Fetching from ", conn)
			conn.(connection.Connection).Fetch(keys, accounts)
	}
}

func ingestAccounts(alib lib.Library, accounts chan data.Account, wg *sync.WaitGroup) {
	defer wg.Done()
	i := 0
	for k := range accounts {
		i++
		alib.Store(k)
	}

	fmt.Printf("Discovered %d accounts\n", i)
}

func ingestKeys(klib lib.Library, keys chan data.Key, wg *sync.WaitGroup) {
	defer wg.Done()
	i := 0
	for k := range keys {
		i++
		klib.Store(k)
	}

	fmt.Printf("Discovered %d keys\n", i)
}