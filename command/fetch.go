package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/urfave/cli"
	"sync"
	"reflect"
)

func CmdFetch(c *cli.Context) error {
	libWG := sync.WaitGroup{}
	ml := lib.MainLibrary{Path: datadir()}

	fKeys := data.NewFanInKey(nil)
	fAccounts := data.NewFanInAccount()

	libWG.Add(1)
	go ingestKeys(ml.Keys(), fKeys.Output(), &libWG)

	libWG.Add(1)
	go ingestAccounts(ml.Accounts(), fAccounts.Output(), &libWG)

	filter := buildFilter(c.Args())

	for conn := range ml.Connections().List() {
		if filter(conn) {
			fmt.Printf("Fetching from %s\n", conn)
			k, a := fetchFrom(conn)
			fKeys.Add(k)
			fAccounts.Add(a)
		}
	}

	fKeys.Wait()
	fAccounts.Wait()
	libWG.Wait()
	return nil
}

func fetchFrom(conn interface{}) (keys chan data.Key, accounts chan data.Account) {
	switch conn.(type) {
	case connection.Connection:
		//fmt.Println("Fetching from ", conn)
		return conn.(connection.Connection).Fetch()
	default:
		panic("Unknown connection type " + reflect.TypeOf(conn).Name())
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
		id := klib.Id(k)
		if existing, err := klib.Fetch(id); err == nil {
			existing.(data.Key).Merge(k)
			klib.Store(existing)
		} else {
			klib.Store(k)
		}
	}

	fmt.Printf("Discovered %d keys\n", i)
}
