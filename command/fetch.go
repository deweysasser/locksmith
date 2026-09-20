package command

import (
	"context"
	"fmt"
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/history"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func CmdFetch(c *cli.Context) error {
	outputLevel(c)
	libWG := sync.WaitGroup{}
	ml := lib.MainLibrary{Path: datadir(c)}

	ctx, cancel := fetchContext(c)
	defer cancel()

	log := history.Open(datadir(c), "fetch")
	defer log.Close()

	fKeys := data.NewFanInKey(nil)
	fAccounts := data.NewFanInAccount()

	libWG.Add(1)
	go ingestKeys(ml.Keys(), fKeys.Output(), &libWG, log)

	libWG.Add(1)
	go ingestAccounts(ml.Accounts(), fAccounts.Output(), &libWG, log)

	filter := buildFilterFromContext(c)

	var connCount int

	for conn := range ml.Connections().List() {
		if filter(conn) {
			output.Verbosef("Fetching from %s\n", conn)
			connCount++
			k, a := fetchFrom(ctx, conn)
			fKeys.Add(k)
			fAccounts.Add(a)
		}
	}

	fKeys.Wait()
	fAccounts.Wait()
	libWG.Wait()

	output.Normalf("Fetched from %d connections\n", connCount)

	return nil
}

// fetchContext builds the context every connection fetches under.
//
// Two ways out: a deadline, so one unresponsive host cannot pin an entire
// fleet run, and an interrupt, so Ctrl-C ends the fetch rather than killing
// the process and orphaning its ssh children.
func fetchContext(c *cli.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	if timeout := c.Duration("timeout"); timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-signals:
			output.Warn("Interrupted; finishing with what has been found so far")
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(signals)
	}()

	return ctx, cancel
}

func fetchFrom(ctx context.Context, conn interface{}) (keys <-chan data.Key, accounts <-chan data.Account) {
	if c, ok := conn.(connection.Connection); ok {
		return c.Fetch(ctx)
	}

	// Reachable whenever connections/ holds an object that deserializes to
	// something that is not a Connection -- a hand-edited repository, or one
	// written by a newer locksmith.  Report it and carry on rather than
	// aborting a fleet-wide fetch.
	output.Error("Not a connection, ignoring:", conn)

	dead := make(chan data.Key)
	close(dead)
	deadAccounts := make(chan data.Account)
	close(deadAccounts)
	return dead, deadAccounts
}

// bindingSet collects an account's bindings for comparison. KeyBindingImpl is
// all strings, so it is comparable and usable as a map key directly.
func bindingSet(a data.Account) map[data.KeyBindingImpl]bool {
	set := make(map[data.KeyBindingImpl]bool)
	for b := range a.Bindings() {
		set[b] = true
	}
	return set
}

// recordBindingChanges writes one event per binding that appeared or vanished.
// Nothing is written when the two sets agree, which on a steady fleet is almost
// every account on almost every run.
func recordBindingChanges(log *history.Log, account data.ID, before, after map[data.KeyBindingImpl]bool) {
	for b := range after {
		if !before[b] {
			log.Record(history.Event{
				Event: history.BindingAdded, Key: b.KeyID,
				Account: account, Location: b.Location,
			})
		}
	}
	for b := range before {
		if !after[b] {
			log.Record(history.Event{
				Event: history.BindingRemoved, Key: b.KeyID,
				Account: account, Location: b.Location,
			})
		}
	}
}

func ingestAccounts(alib lib.AccountLibrary, accounts chan data.Account, wg *sync.WaitGroup, log *history.Log) {
	defer wg.Done()
	idmap := make(map[data.ID]bool)
	i := 0
	for k := range accounts {
		i++
		id := alib.Id(k)
		idmap[id] = true
		if existing, err := alib.Fetch(id); err == nil {
			if existingacct, ok := existing.(data.Account); ok {
				before := bindingSet(existingacct)
				existingacct.Merge(k)
				recordBindingChanges(log, id, before, bindingSet(existingacct))
				if e := alib.Store(existingacct); e != nil {
					output.Error(e)
				}
			} else {
				panic(fmt.Sprint("type for", id, " was not Account"))
			}
		} else if accountHasBindings(k) {
			recordBindingChanges(log, id, nil, bindingSet(k))
			if e := alib.Store(k); e != nil {
				output.Error(e)
			}
		} else {
			// A previously unknown account with no keys is not worth a record:
			// a Linux host has dozens of system accounts that will never hold
			// one.  Connections still report them, because an account we
			// *already* know about has to be reported even when empty -- that
			// is the only way bindings recorded for it can ever be cleared.
			output.Debug("Ignoring new account with no keys:", k.Id())
			delete(idmap, id)
		}
	}

	output.Normalf("Discovered %d accounts in %d references\n", len(idmap), i)
}

// accountHasBindings reports whether the account carries at least one key
// binding.  Bindings() hands back a channel, so this drains it; that is only
// acceptable because it runs on the cold path, for accounts not already known.
func accountHasBindings(a data.Account) bool {
	for range a.Bindings() {
		return true
	}
	return false
}

func ingestKeys(klib lib.KeyLibrary, keys chan data.Key, wg *sync.WaitGroup, log *history.Log) {
	defer wg.Done()
	idmap := make(map[data.ID]bool)
	i := 0
	for k := range keys {
		i++
		id := klib.Id(k)
		idmap[id] = true
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
			// Not in the library under any of its identifiers: this is the
			// first time locksmith has ever seen this key.
			names := k.GetNames()
			log.Record(history.Event{
				Event: history.KeyDiscovered, Key: id,
				KeyName: names.Join(", "),
			})
			if e := klib.Store(k); e != nil {
				output.Error(e)
			}
		}
	}

	output.Normalf("Discovered %d keys in %d locations\n", len(idmap), i)
}
