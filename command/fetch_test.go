package command

import (
	"context"
	"encoding/json"
	"github.com/deweysasser/locksmith/history"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
)

// runIngestKeys feeds the given keys through ingestKeys and returns the library
// it filled.
func runIngestKeys(t *testing.T, keys ...data.Key) lib.KeyLibrary {
	t.Helper()

	ml := lib.MainLibrary{Path: t.TempDir()}
	klib := ml.Keys()

	c := make(chan data.Key)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go ingestKeys(klib, c, &wg, nil)

	for _, k := range keys {
		c <- k
	}
	close(c)
	wg.Wait()

	return klib
}

func countKeys(l lib.KeyLibrary) int {
	n := 0
	for range l.List() {
		n++
	}
	return n
}

func TestIngestKeysStoresNewKeys(t *testing.T) {
	silence(t)

	klib := runIngestKeys(t,
		data.NewAwsKey("AKIA1", time.Time{}, true, "one"),
		data.NewAwsKey("AKIA2", time.Time{}, true, "two"),
	)

	if got := countKeys(klib); got != 2 {
		t.Errorf("stored %d keys, want 2", got)
	}
}

// The same key turning up on two hosts is one key with two names, not two keys.
func TestIngestKeysMergesRepeatSightings(t *testing.T) {
	silence(t)

	early := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

	klib := runIngestKeys(t,
		data.NewAwsKey("AKIA1", late, true, "from-host-a"),
		data.NewAwsKey("AKIA1", early, true, "from-host-b"),
	)

	if got := countKeys(klib); got != 1 {
		t.Fatalf("stored %d keys, want 1", got)
	}

	got, err := klib.Fetch("AKIA1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	names := got.GetNames()
	if !names.Contains("from-host-a") || !names.Contains("from-host-b") {
		t.Errorf("names %v should record both sightings", names.StringArray())
	}
}

// Deprecation set in one run has to survive the next fetch, or `expire` would
// be undone by the next `fetch`.
func TestIngestKeysKeepsDeprecation(t *testing.T) {
	silence(t)

	ml := lib.MainLibrary{Path: t.TempDir()}
	klib := ml.Keys()

	expired := data.NewAwsKey("AKIA1", time.Time{}, true, "old")
	expired.Expire()
	if err := klib.Store(expired); err != nil {
		t.Fatalf("Store: %v", err)
	}

	c := make(chan data.Key)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go ingestKeys(klib, c, &wg, nil)
	c <- data.NewAwsKey("AKIA1", time.Time{}, true, "rediscovered")
	close(c)
	wg.Wait()

	got, err := klib.Fetch("AKIA1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !got.IsDeprecated() {
		t.Error("re-fetching a key should not undo its deprecation")
	}
}

func TestIngestKeysWithNothingToIngest(t *testing.T) {
	silence(t)

	if got := countKeys(runIngestKeys(t)); got != 0 {
		t.Errorf("stored %d keys, want 0", got)
	}
}

func TestIngestAccountsStoresAndMerges(t *testing.T) {
	silence(t)

	ml := lib.MainLibrary{Path: t.TempDir()}
	alib := ml.Accounts()

	c := make(chan data.Account)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go ingestAccounts(alib, c, &wg, nil)

	c <- data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: "key1", Location: data.AUTHORIZED_KEYS},
	})
	c <- data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: "key2", Location: data.AUTHORIZED_KEYS},
	})
	// A previously unknown account with no keys is not worth a record: a host
	// has dozens of system accounts that will never hold one.
	c <- data.NewSSHAccount("root", "other.example.com", "conn1", nil)
	close(c)
	wg.Wait()

	n := 0
	for range alib.List() {
		n++
	}
	if n != 1 {
		t.Fatalf("stored %d accounts, want 1 (the keyless new account is not recorded)", n)
	}

	got, err := alib.Fetch("root@host.example.com")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	var bindings []data.KeyBindingImpl
	for b := range got.Bindings() {
		bindings = append(bindings, b)
	}
	if len(bindings) != 2 {
		t.Errorf("merged account has %d bindings, want both keys", len(bindings))
	}
}

func TestFetchFromDispatchesOnConnectionType(t *testing.T) {
	silence(t)

	conn := &connection.FileConnection{Type: "FileConnection", Path: "../data/test-data/rsa.pub"}

	keyChan, acctChan := fetchFrom(context.Background(), conn)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range acctChan {
		}
	}()

	n := 0
	for range keyChan {
		n++
	}
	<-done

	if n != 1 {
		t.Errorf("fetched %d keys, want 1", n)
	}
}

// A connections/ directory can hold an object that is not a Connection: a
// hand-edited repository, a bad git merge, or one written by a newer locksmith.
// That used to panic, taking down a fetch across the whole fleet over one bad
// file. It is reported and skipped instead, and the caller still gets closed
// channels so the fan-in completes.
func TestFetchFromRejectsNonConnections(t *testing.T) {
	silence(t)

	keys, accounts := fetchFrom(context.Background(), struct{ Name string }{"not a connection"})

	for range keys {
		t.Error("a non-connection yielded a key")
	}
	for range accounts {
		t.Error("a non-connection yielded an account")
	}
}

// silence keeps the leveled logger quiet for the duration of a test.
func silence(t *testing.T) {
	t.Helper()
	saved := output.Level
	output.Level = output.SilentLevel
	t.Cleanup(func() { output.Level = saved })
}

// The other half of the rule: an account we already know about IS reported and
// merged even when it now has no keys, because that is the only way bindings
// recorded for it can ever be cleared.  Without this a key removed from a host
// is reported as still present forever.
func TestIngestAccountsClearsBindingsWhenAKeyIsRemoved(t *testing.T) {
	silence(t)

	ml := lib.MainLibrary{Path: t.TempDir()}
	alib := ml.Accounts()

	fetch := func(keys ...data.ID) {
		var bindings []data.KeyBindingImpl
		for _, k := range keys {
			bindings = append(bindings, data.KeyBindingImpl{KeyID: k, Location: data.AUTHORIZED_KEYS})
		}
		acct := data.NewSSHAccount("root", "host.example.com", "conn1", bindings)
		acct.MarkObserved(data.AUTHORIZED_KEYS)

		c := make(chan data.Account)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go ingestAccounts(alib, c, &wg, nil)
		c <- acct
		close(c)
		wg.Wait()
	}

	bound := func() []data.ID {
		acct, err := alib.Fetch("root@host.example.com")
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		var got []data.ID
		for b := range acct.Bindings() {
			got = append(got, b.KeyID)
		}
		return got
	}

	fetch("keyA", "keyB")
	if got := bound(); len(got) != 2 {
		t.Fatalf("after the first fetch the account has %v, want both keys", got)
	}

	// keyB has been removed from the host.
	fetch("keyA")
	got := bound()
	if len(got) != 1 || got[0] != "keyA" {
		t.Fatalf("after keyB was removed the account still records %v, want only keyA", got)
	}

	// And authorized_keys emptied entirely.
	fetch()
	if got := bound(); len(got) != 0 {
		t.Errorf("after authorized_keys was emptied the account still records %v", got)
	}
}

// A connection that did not claim authority must not have its bindings dropped:
// one account legitimately carries bindings from several sources.
func TestIngestAccountsKeepsBindingsFromUnclaimedLocations(t *testing.T) {
	silence(t)

	ml := lib.MainLibrary{Path: t.TempDir()}
	alib := ml.Accounts()

	send := func(acct data.Account) {
		c := make(chan data.Account)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go ingestAccounts(alib, c, &wg, nil)
		c <- acct
		close(c)
		wg.Wait()
	}

	initial := data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: "authorized", Location: data.AUTHORIZED_KEYS},
		{KeyID: "instance", Location: data.INSTANCE_ROOT_CREDENTIALS},
	})
	send(initial)

	// A fetch that surveyed only authorized_keys, and found nothing there.
	empty := data.NewSSHAccount("root", "host.example.com", "conn1", nil)
	empty.MarkObserved(data.AUTHORIZED_KEYS)
	send(empty)

	acct, err := alib.Fetch("root@host.example.com")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	var got []data.ID
	for b := range acct.Bindings() {
		got = append(got, b.KeyID)
	}

	if len(got) != 1 || got[0] != "instance" {
		t.Errorf("bindings = %v, want only the instance binding kept", got)
	}
}

// The history log is the only record of what changed, so it has to be written
// by the real ingestion path, not just by the history package's own tests.
func TestIngestRecordsTransitionsToHistory(t *testing.T) {
	silence(t)

	repo := t.TempDir()
	ml := lib.MainLibrary{Path: repo}
	alib := ml.Accounts()

	fetch := func(log *history.Log, keys ...data.ID) {
		var bindings []data.KeyBindingImpl
		for _, k := range keys {
			bindings = append(bindings, data.KeyBindingImpl{KeyID: k, Location: data.AUTHORIZED_KEYS})
		}
		acct := data.NewSSHAccount("root", "host.example.com", "conn1", bindings)
		acct.MarkObserved(data.AUTHORIZED_KEYS)

		c := make(chan data.Account)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go ingestAccounts(alib, c, &wg, log)
		c <- acct
		close(c)
		wg.Wait()
	}

	events := func(log *history.Log) []string {
		if log.Path() == "" {
			return nil
		}
		body, err := os.ReadFile(log.Path())
		if err != nil {
			t.Fatalf("reading history: %v", err)
		}
		var got []string
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
			if line == "" {
				continue
			}
			var e history.Event
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				t.Fatalf("history line %q: %v", line, err)
			}
			got = append(got, e.Event+" "+string(e.Key))
		}
		sort.Strings(got)
		return got
	}

	first := history.Open(repo, "fetch")
	fetch(first, "keyA", "keyB")
	first.Close()
	if got := events(first); len(got) != 2 || got[0] != "binding.added keyA" || got[1] != "binding.added keyB" {
		t.Errorf("first fetch recorded %v, want both bindings added", got)
	}

	// Nothing changed: the log must stay empty, and leave no file at all.
	steady := history.Open(repo, "fetch")
	fetch(steady, "keyA", "keyB")
	steady.Close()
	if got := events(steady); len(got) != 0 {
		t.Errorf("an unchanged fetch recorded %v, want nothing", got)
	}
	if steady.Path() != "" {
		t.Error("an unchanged fetch left a history file behind")
	}

	// keyB removed from the host.
	removal := history.Open(repo, "fetch")
	fetch(removal, "keyA")
	removal.Close()
	if got := events(removal); len(got) != 1 || got[0] != "binding.removed keyB" {
		t.Errorf("removal recorded %v, want binding.removed keyB", got)
	}
}

// A key seen for the first time is recorded; seeing it again is not.
func TestIngestKeysRecordsDiscoveryOnce(t *testing.T) {
	silence(t)

	repo := t.TempDir()
	ml := lib.MainLibrary{Path: repo}
	klib := ml.Keys()

	send := func(log *history.Log) {
		c := make(chan data.Key)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go ingestKeys(klib, c, &wg, log)
		c <- data.NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
		close(c)
		wg.Wait()
	}

	first := history.Open(repo, "fetch")
	send(first)
	first.Close()
	if first.Path() == "" {
		t.Fatal("discovering a key recorded nothing")
	}

	again := history.Open(repo, "fetch")
	send(again)
	again.Close()
	if again.Path() != "" {
		t.Error("re-seeing a known key recorded a discovery")
	}
}
