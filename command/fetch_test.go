package command

import (
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
	go ingestKeys(klib, c, &wg)

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
	go ingestKeys(klib, c, &wg)
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
	go ingestAccounts(alib, c, &wg)

	c <- data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: "key1", Location: data.AUTHORIZED_KEYS},
	})
	c <- data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: "key2", Location: data.AUTHORIZED_KEYS},
	})
	c <- data.NewSSHAccount("root", "other.example.com", "conn1", nil)
	close(c)
	wg.Wait()

	n := 0
	for range alib.List() {
		n++
	}
	if n != 2 {
		t.Fatalf("stored %d accounts, want 2", n)
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

	keyChan, acctChan := fetchFrom(conn)

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

func TestFetchFromRejectsNonConnections(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("fetching from something that is not a connection should panic")
		}
	}()

	fetchFrom(struct{ Name string }{"not a connection"})
}

// silence keeps the leveled logger quiet for the duration of a test.
func silence(t *testing.T) {
	t.Helper()
	saved := output.Level
	output.Level = output.SilentLevel
	t.Cleanup(func() { output.Level = saved })
}
