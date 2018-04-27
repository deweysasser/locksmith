package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/lib"
	"github.com/urfave/cli"
	"sync"
	"github.com/deweysasser/locksmith/output"
	"github.com/deweysasser/locksmith/data"
)

func CmdList(c *cli.Context) error {

	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir()}

	wg := sync.WaitGroup{}
	wg.Add(1)
	filter := buildFilter(c)
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

	for i := range ml.Connections().List() {
		ch <- connectionString(i, "")
		if output.IsLevel(output.VerboseLevel) {
		}
	}

	accountsForKey := make(map[data.ID][]data.ID)

	for i := range ml.Accounts().List() {
		ch <- accountString(i, "")
		if output.IsLevel(output.VerboseLevel) {
			outputKeysFor(ch, i.(data.Account), accountsForKey, ml.Keys())
		}
	}
	for i := range ml.Keys().List() {
		ch <- keyString(i, "")
		if output.IsLevel(output.DebugLevel) {
			for _, id := range i.(data.Key).Identifiers() {
				ch <- fmt.Sprintf("  ID %s", id)
			}
		}
	}

	return nil
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

func outputKeysFor(ch chan <-string, a data.Account, m map[data.ID][]data.ID, keys lib.Library) {
	for _, k:=range a.Bindings() {
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
