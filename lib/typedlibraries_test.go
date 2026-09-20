package lib

import (
	"testing"
	"time"

	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
)

func TestAccountLibraryRoundTrip(t *testing.T) {
	lib := NewAccountLibrary(t.TempDir())

	acct := data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: "key1", Location: data.AUTHORIZED_KEYS},
	})

	if err := lib.Store(acct); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, err := lib.Fetch(acct.Id())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.Id() != acct.Id() {
		t.Errorf("fetched account ID = %q, want %q", got.Id(), acct.Id())
	}
	if got.ConnectionID() != "conn1" {
		t.Errorf("fetched connection ID = %q, want conn1", got.ConnectionID())
	}

	var bindings []data.KeyBindingImpl
	for b := range got.Bindings() {
		bindings = append(bindings, b)
	}
	if len(bindings) != 1 || bindings[0].KeyID != "key1" {
		t.Errorf("fetched bindings = %v, want one binding to key1", bindings)
	}
}

func TestAccountLibraryList(t *testing.T) {
	lib := NewAccountLibrary(t.TempDir())

	for _, host := range []string{"a.example.com", "b.example.com"} {
		if err := lib.Store(data.NewSSHAccount("root", host, "conn1", nil)); err != nil {
			t.Fatalf("Store: %v", err)
		}
	}

	count := 0
	for range lib.List() {
		count++
	}

	if count != 2 {
		t.Errorf("listed %d accounts, want 2", count)
	}
}

func TestAccountLibraryListMatching(t *testing.T) {
	lib := NewAccountLibrary(t.TempDir())

	for _, host := range []string{"a.example.com", "b.example.com"} {
		if err := lib.Store(data.NewSSHAccount("root", host, "conn1", nil)); err != nil {
			t.Fatalf("Store: %v", err)
		}
	}

	count := 0
	for a := range lib.ListMatching(func(a data.Account) bool { return a.Id() == "root@a.example.com" }) {
		if a.Id() != "root@a.example.com" {
			t.Errorf("predicate let through %q", a.Id())
		}
		count++
	}

	if count != 1 {
		t.Errorf("listed %d accounts, want 1", count)
	}
}

func TestAccountLibraryDelete(t *testing.T) {
	lib := NewAccountLibrary(t.TempDir())

	acct := data.NewSSHAccount("root", "host.example.com", "conn1", nil)
	if err := lib.Store(acct); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := lib.DeleteObject(acct); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	if _, err := lib.Fetch(acct.Id()); err == nil {
		t.Error("a deleted account should not be fetchable")
	}
}

func TestConnectionLibraryRoundTrip(t *testing.T) {
	lib := NewConnectionLibrary(t.TempDir())

	conn := &connection.SSHHostConnection{
		Type:       "SSHHostConnection",
		Connection: "ubuntu@host.example.com",
		Sudo:       true,
	}

	if err := lib.Store(conn); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, err := lib.Fetch(conn.Id())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	restored, ok := got.(*connection.SSHHostConnection)
	if !ok {
		t.Fatalf("fetched %T, want *connection.SSHHostConnection", got)
	}
	if restored.Connection != "ubuntu@host.example.com" {
		t.Errorf("Connection = %q, want ubuntu@host.example.com", restored.Connection)
	}
	if !restored.Sudo {
		t.Error("the sudo flag should survive a round trip")
	}
}

func TestConnectionLibraryHoldsEveryConnectionKind(t *testing.T) {
	lib := NewConnectionLibrary(t.TempDir())

	conns := []connection.Connection{
		&connection.SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@host.example.com"},
		&connection.FileConnection{Type: "FileConnection", Path: "/home/someone/.ssh"},
		&connection.AWSConnection{Type: "AWSConnection", Profile: "default"},
	}

	for _, c := range conns {
		if err := lib.Store(c); err != nil {
			t.Fatalf("storing %T: %v", c, err)
		}
	}

	count := 0
	for range lib.List() {
		count++
	}

	if count != len(conns) {
		t.Errorf("listed %d connections, want %d", count, len(conns))
	}
}

func TestKeyLibraryRoundTrip(t *testing.T) {
	lib := NewKeyLibrary(t.TempDir())

	key := data.NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
	if err := lib.Store(key); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, err := lib.Fetch("AKIAEXAMPLE")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.Id() != key.Id() {
		t.Errorf("fetched key ID = %q, want %q", got.Id(), key.Id())
	}
}

func TestKeyLibraryDelete(t *testing.T) {
	lib := NewKeyLibrary(t.TempDir())

	key := data.NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
	if err := lib.Store(key); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := lib.Delete("AKIAEXAMPLE"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := lib.Fetch("AKIAEXAMPLE"); err == nil {
		t.Error("a deleted key should not be fetchable")
	}
}

func TestKeyLibraryListMatching(t *testing.T) {
	lib := NewKeyLibrary(t.TempDir())

	live := data.NewAwsKey("AKIALIVE", time.Time{}, true, "prod")
	dead := data.NewAwsKey("AKIADEAD", time.Time{}, true, "old")
	dead.Expire()

	for _, k := range []data.Key{live, dead} {
		if err := lib.Store(k); err != nil {
			t.Fatalf("Store: %v", err)
		}
	}

	var got []data.ID
	for k := range lib.ListMatching(func(k data.Key) bool { return !k.IsDeprecated() }) {
		got = append(got, k.Id())
	}

	if len(got) != 1 || got[0] != "AKIALIVE" {
		t.Errorf("listed %v, want only the live key", got)
	}
}

func TestChangeLibraryRoundTrip(t *testing.T) {
	lib := NewChangeLibrary(t.TempDir())

	change := data.Change{
		Type:    "Change",
		Account: "root@host.example.com",
		Add:     []data.KeyBindingImpl{{KeyID: "newkey", Location: data.AUTHORIZED_KEYS}},
		Remove:  []data.KeyBindingImpl{{KeyID: "oldkey", Location: data.AUTHORIZED_KEYS}},
	}

	if err := lib.Store(change); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, err := lib.Fetch("root@host.example.com")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.Account != change.Account {
		t.Errorf("fetched account = %q, want %q", got.Account, change.Account)
	}
	if len(got.Add) != 1 || got.Add[0].KeyID != "newkey" {
		t.Errorf("fetched additions = %v, want one addition of newkey", got.Add)
	}
	if len(got.Remove) != 1 || got.Remove[0].KeyID != "oldkey" {
		t.Errorf("fetched removals = %v, want one removal of oldkey", got.Remove)
	}
}

// A change is identified by the account it applies to, so re-planning an
// account replaces its pending change rather than stacking up another one.
func TestChangeLibraryKeepsOneChangePerAccount(t *testing.T) {
	lib := NewChangeLibrary(t.TempDir())

	first := data.Change{
		Type:    "Change",
		Account: "root@host.example.com",
		Add:     []data.KeyBindingImpl{{KeyID: "key1"}},
	}
	second := data.Change{
		Type:    "Change",
		Account: "root@host.example.com",
		Add:     []data.KeyBindingImpl{{KeyID: "key2"}},
	}

	if err := lib.Store(first); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := lib.Store(second); err != nil {
		t.Fatalf("Store: %v", err)
	}

	var got []data.Change
	for c := range lib.List() {
		got = append(got, c)
	}

	if len(got) != 1 {
		t.Fatalf("listed %d changes for one account, want 1", len(got))
	}
	if len(got[0].Add) != 1 || got[0].Add[0].KeyID != "key2" {
		t.Errorf("stored change = %v, want the most recent plan", got[0])
	}
}

func TestChangeLibraryDelete(t *testing.T) {
	lib := NewChangeLibrary(t.TempDir())

	change := data.Change{
		Type:    "Change",
		Account: "root@host.example.com",
		Add:     []data.KeyBindingImpl{{KeyID: "newkey"}},
	}

	if err := lib.Store(change); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := lib.DeleteObject(change); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	count := 0
	for range lib.List() {
		count++
	}
	if count != 0 {
		t.Errorf("listed %d changes after deleting the only one, want 0", count)
	}
}

// apply lists changes and then deletes the ones it managed to apply, so a
// change that came back off disk has to be deletable.
func TestChangeLibraryDeleteAfterList(t *testing.T) {
	dir := t.TempDir()
	lib := NewChangeLibrary(dir)

	if err := lib.Store(data.Change{
		Type:    "Change",
		Account: "root@host.example.com",
		Add:     []data.KeyBindingImpl{{KeyID: "newkey"}},
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// A fresh library, as a later `apply` invocation would see it.
	reopened := NewChangeLibrary(dir)
	for c := range reopened.List() {
		if err := reopened.DeleteObject(c); err != nil {
			t.Fatalf("DeleteObject: %v", err)
		}
	}

	count := 0
	for range NewChangeLibrary(dir).List() {
		count++
	}
	if count != 0 {
		t.Errorf("listed %d changes after applying them all, want 0", count)
	}
}
