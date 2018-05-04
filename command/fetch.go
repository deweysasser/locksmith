package command

import (
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"reflect"
	"sync"
	"fmt"
)

func CmdFetch(c *cli.Context) error {
	outputLevel(c)
	libWG := sync.WaitGroup{}
	ml := lib.MainLibrary{Path: datadir(c)}

	fKeys := data.NewFanInKey(nil)
	fAccounts := data.NewFanInAccount()

	libWG.Add(1)
	go ingestKeys(ml.Keys(), fKeys.Output(), &libWG)

	libWG.Add(1)
	go ingestAccounts(ml.Accounts(), fAccounts.Output(), &libWG)

	filter := buildFilterFromContext(c)

	for conn := range ml.Connections().List() {
		if filter(conn) {
			output.Verbosef("Fetching from %s\n", conn)
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

func fetchFrom(conn interface{}) (keys <-  chan data.Key, accounts <- chan data.Account) {
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
	idmap := make(map[string] bool)
	i := 0
	for k := range accounts {
		i++
		id := alib.Id(k)
		idmap[id]=true
		if existing, err := alib.Fetch(id); err == nil {
			if existingacct, ok := existing.(data.Account); ok {
				existingacct.Merge(k)
				if e := alib.Store(existingacct); e != nil {
					output.Error(e)
				}
			} else {
				panic(fmt.Sprint("type for", id, " was not Account"))
			}
		} else {
			if e := alib.Store(k); e != nil {
				output.Error(e)
			}
		}
	}

	output.Normalf("Discovered %d accounts in %d references\n", len(idmap), i)
}

func ingestKeys(klib lib.Library, keys chan data.Key, wg *sync.WaitGroup) {
	defer wg.Done()
	idmap := make(map[string] bool)
	i := 0
	for k := range keys {
		i++
		id := klib.Id(k)
		idmap[id]=true
		if existing, err := klib.Fetch(id); err == nil {
			existing.(data.Key).Merge(k)
			if e := klib.Store(existing); e != nil {
				output.Error(e)
			}
			// It's possible for a key primary ID to change if we didn't before have a public key.
			if klib.Id(existing) != id {
				output.Debug("Updating key id from", id, "to", klib.Id(existing))
				// If so, delete the previous key file.  This, however, takes they key out of the cache so we need to
				// re-cache it.  Storing it again puts it back in the cache at the cost of a bit more disk I/O (but code
				// simplicity)
				if e := klib.Delete(id); e == nil {
					if e := klib.Store(existing); e != nil {
						output.Error("Error re-storing", klib.Id(existing))
					}
				} 
			}

		} else {
			if e := klib.Store(k); e != nil {
				output.Error(e)
			}
		}
	}

	output.Normalf("Discovered %d keys in %d locations\n", len(idmap), i)
}
