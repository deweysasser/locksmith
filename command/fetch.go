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
	ml := lib.MainLibrary{Path: datadir()}

	keys := make(chan data.Key)
	done := make(chan interface{})

	go ingest(ml.Keys(), keys, done)
	filter := buildFilter(c.Args())

	wg:=sync.WaitGroup{}
	for conn := range ml.Connections().List() {
		if filter(conn) {
			wg.Add(1)
			go func (c interface{}) {
				defer wg.Done()
				fetchFrom(conn, keys)
			}(conn)
		}
	}

	wg.Wait()
	close(keys)
	<- done
	return nil
}

func fetchFrom(conn interface{}, keys chan data.Key) {
	switch conn.(type) {
	case connection.Connection:
			//fmt.Println("Fetching from ", conn)
			conn.(connection.Connection).Fetch(keys)
	}
}

func ingest(klib lib.Library, keys chan data.Key, done chan interface{}) {
	i := 0
	for k := range keys {
		i++
		klib.Store(k)
	}

	fmt.Printf("Discovered %d keys\n", i)
	done <- nil
}