package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"sync"
)

func CmdList(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir()}

	wg := sync.WaitGroup{}
	wg.Add(1)
	filter := buildFilterFromContext(c)
	ch := make(chan string)

	// Start the printer
	go func() {
		defer wg.Done()
		for s := range ch {
			if filter(s) {
				output.Normal(s)
			}
		}
	}()

	printConnections(ml.Connections(), ch)
	printAccounts(ml.Accounts(), ch, ml)
	printKeys(ml.Keys(), ch)

	close(ch)
	wg.Wait()

	return nil
}

func printKeys(keys lib.Library, ch chan<- string) {
	for i := range keys.List() {
		ch <- keyString(i, "")
		if output.IsLevel(output.DebugLevel) {
			for _, id := range i.(data.Key).Identifiers() {
				ch <- fmt.Sprintf("  ID %s", id)
			}
		}
	}
}

func printAccounts(accounts lib.Library, ch chan<- string, ml lib.MainLibrary) map[data.ID][]data.ID {
	accountsForKey := make(map[data.ID][]data.ID)
	for i := range accounts.List() {
		ch <- accountString(i, "")
		if output.IsLevel(output.VerboseLevel) {
			outputKeysFor(ch, i.(data.Account), accountsForKey, ml.Keys())
		}
	}

	return accountsForKey
}

func printConnections(connections lib.Library, ch chan<- string) {
	for i := range connections.List() {
		ch <- connectionString(i, "")
		if output.IsLevel(output.VerboseLevel) {
		}
	}
}

func connectionString(i interface{}, prefix string) string {
	return fmt.Sprintf("%sconnection %s", prefix, i)
}

func accountString(i interface{}, prefix string) string {
	return fmt.Sprintf("%saccount %s", prefix, i)
}

func keyString(i interface{}, prefix string) string {
	return fmt.Sprintf("%skey %s", prefix, i)
}

func outputKeysFor(ch chan<- string, a data.Account, m map[data.ID][]data.ID, keys lib.Library) {
	for _, k := range a.Bindings() {
		if s, key := k.Describe(keys); key != nil {
			if k, ok := key.(data.Key); ok {
				m[k.Id()] = append(m[k.Id()], a.Id())
			} else {
				output.Error("Key ID", key, "was not a Key")
			}
			ch <- "  " + s
		} else {
			ch <- "  " + s
		}
	}
}
