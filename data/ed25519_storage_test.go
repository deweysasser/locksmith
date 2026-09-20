// This file is an external test package so that it can import lib/, which
// imports data/ and so cannot be imported from inside package data.
package data_test

import (
	"testing"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
)

// edtestStoredKey reads the Ed25519 fixture as package data would.
func edtestStoredKey(t *testing.T) *data.SSHKey {
	t.Helper()

	key := data.Read("test-data/ed25519.pub")
	if key == nil {
		t.Fatal("reading the Ed25519 fixture returned nil")
	}
	sshKey, ok := key.(*data.SSHKey)
	if !ok {
		t.Fatalf("fixture parsed as %T, want *data.SSHKey", key)
	}
	return sshKey
}

// An Ed25519 key has to survive the full trip through the on-disk library:
// serialize under its "SSHKey" type name, come back through the TypeMap, and
// still answer to every identifier it was stored under.
func TestEd25519RoundTripsThroughLibrary(t *testing.T) {
	dir := t.TempDir()
	key := edtestStoredKey(t)
	key.Names.Add("ed25519.pub")

	if err := lib.NewKeyLibrary(dir).Store(key); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// A fresh library has an empty cache, so this reads and deserializes the
	// file rather than handing back the object we just stored.
	reloaded := lib.NewKeyLibrary(dir)

	for _, id := range key.Identifiers() {
		fetched, err := reloaded.Fetch(id)
		if err != nil {
			t.Fatalf("Fetch(%q): %v", id, err)
		}

		restored, ok := fetched.(*data.SSHKey)
		if !ok {
			t.Fatalf("Fetch(%q) returned %T, want *data.SSHKey", id, fetched)
		}
		if restored.Id() != key.Id() {
			t.Errorf("Fetch(%q).Id() = %q, want %q", id, restored.Id(), key.Id())
		}
		if got := restored.KeyType(); got != "ssh-ed25519" {
			t.Errorf("Fetch(%q).KeyType() = %q, want ssh-ed25519", id, got)
		}
		if got, want := restored.PublicKeyString(), key.PublicKeyString(); got != want {
			t.Errorf("Fetch(%q) public key = %q, want %q", id, got, want)
		}
		if !restored.Comments.Contains("dewey@locksmith-test") {
			t.Errorf("Fetch(%q) comments = %v, want the comment preserved", id, restored.Comments.StringArray())
		}
		if !restored.Names.Contains("ed25519.pub") {
			t.Errorf("Fetch(%q) names = %v, want the name preserved", id, restored.Names.StringArray())
		}
	}
}

// Listing is how `locksmith list` reaches stored keys, and it goes through the
// same type-dispatched deserialization.
func TestEd25519IsListedFromLibrary(t *testing.T) {
	dir := t.TempDir()
	key := edtestStoredKey(t)

	if err := lib.NewKeyLibrary(dir).Store(key); err != nil {
		t.Fatalf("Store: %v", err)
	}

	var found []data.Key
	for k := range lib.NewKeyLibrary(dir).List() {
		found = append(found, k)
	}

	if len(found) != 1 {
		t.Fatalf("got %d keys from the library, want 1", len(found))
	}
	if found[0].Id() != key.Id() {
		t.Errorf("listed key ID = %q, want %q", found[0].Id(), key.Id())
	}
}
