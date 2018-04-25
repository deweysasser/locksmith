package command

import (
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/urfave/cli"
	"sync"
	"reflect"
	"github.com/deweysasser/locksmith/output"
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

	filter := buildFilter(c)

	for conn := range ml.Connections().List() {
		if filter(conn) {
			output.Debugf("Fetching from %s\n", conn)
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

	output.Normalf("Discovered %d accounts\n", i)
}

func ingestKeys(klib lib.Library, keys chan data.Key, wg *sync.WaitGroup) {
	defer wg.Done()
	i := 0
	for k := range keys {
		i++
		id := klib.Id(k)
		if existing, err := klib.Fetch(id); err == nil {
			existing.(data.Key).Merge(k)
			if e := klib.Store(existing); e != nil {
				output.Error(e)
			}
		} else {
			if e:= klib.Store(k); e != nil {
				output.Error(e)
			}
		}
	}

	output.Normalf("Discovered %d keys\n", i)
}
