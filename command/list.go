package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/lib"
	"github.com/urfave/cli"
	"sync"
	"github.com/deweysasser/locksmith/output"
)

func CmdList(c *cli.Context) error {
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
		ch <- fmt.Sprintf("connection %s", i)
	}
	for i := range ml.Accounts().List() {
		ch <- fmt.Sprintf("account %s", i)
	}
	for i := range ml.Keys().List() {
		ch <- fmt.Sprintf("key %s", i)
	}

	return nil
}
