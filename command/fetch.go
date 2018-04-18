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

	keys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	libWG.Add(1)
	go ingestKeys(ml.Keys(), keys, &libWG)


	libWG.Add(1)
	go ingestAccounts(ml.Accounts(), cAccounts, &libWG)

	filter := buildFilter(c.Args())

	wgConnections :=sync.WaitGroup{}
	for conn := range ml.Connections().List() {
		if filter(conn) {
			wgConnections.Add(1)
			go func (c interface{}) {
				defer wgConnections.Done()
				fetchFrom(c, keys, cAccounts)
			}(conn)
		}
	}

	wgConnections.Wait()
	close(keys)
	close(cAccounts)
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