package connection

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deweysasser/locksmith/data"
)

// edtestCopyFixture copies a fixture from data/test-data into dir under the
// given name and returns the directory.
func edtestCopyFixture(t *testing.T, dir, fixture, name string) {
	t.Helper()

	bytes, err := ioutil.ReadFile(filepath.Join("..", "data", "test-data", fixture))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", fixture, err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, name), bytes, 0600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

// edtestSSHKey asserts a fetched key is an SSH key and returns it.
func edtestSSHKey(t *testing.T, key data.Key) *data.SSHKey {
	t.Helper()

	sshKey, ok := key.(*data.SSHKey)
	if !ok {
		t.Fatalf("fetched key is %T, want *data.SSHKey", key)
	}
	return sshKey
}

// A .ssh directory holding an Ed25519 keypair is the common case on a modern
// machine, and both halves have to come back as the same key.
func TestFileConnectionFetchEd25519Directory(t *testing.T) {
	dir := t.TempDir()
	edtestCopyFixture(t, dir, "ed25519", "id_ed25519")
	edtestCopyFixture(t, dir, "ed25519.pub", "id_ed25519.pub")

	keys, accounts := fetchAll(&FileConnection{Type: "FileConnection", Path: dir})

	if len(keys) != 2 {
		t.Fatalf("got %d keys from an Ed25519 keypair, want 2", len(keys))
	}
	if len(accounts) != 0 {
		t.Errorf("a file connection should discover no accounts, got %d", len(accounts))
	}

	names := make(map[string]bool)
	var id data.ID
	for _, k := range keys {
		sshKey := edtestSSHKey(t, k)
		if got := sshKey.KeyType(); got != "ssh-ed25519" {
			t.Errorf("KeyType() = %q, want ssh-ed25519", got)
		}
		if id == "" {
			id = sshKey.Id()
		} else if sshKey.Id() != id {
			t.Errorf("the two halves of one keypair have different IDs: %q and %q", id, sshKey.Id())
		}
		keyNames := sshKey.GetNames()
		for _, n := range keyNames.StringArray() {
			names[n] = true
		}
	}

	for _, want := range []string{"id_ed25519", "id_ed25519.pub"} {
		if !names[want] {
			t.Errorf("key names %v should include %q", names, want)
		}
	}
}

func TestFileConnectionFetchEd25519PublicKeyFile(t *testing.T) {
	c := &FileConnection{Type: "FileConnection", Path: "../data/test-data/ed25519.pub"}

	keys, _ := fetchAll(c)

	if len(keys) != 1 {
		t.Fatalf("got %d keys from a single Ed25519 public key file, want 1", len(keys))
	}

	key := edtestSSHKey(t, keys[0])
	if got := key.KeyType(); got != "ssh-ed25519" {
		t.Errorf("KeyType() = %q, want ssh-ed25519", got)
	}
	if !key.Comments.Contains("dewey@locksmith-test") {
		t.Errorf("comments = %v, want the fixture comment", key.Comments.StringArray())
	}
	keyNames := key.GetNames()
	if !keyNames.Contains("ed25519.pub") {
		t.Errorf("key names %v should include the file name", keyNames.StringArray())
	}
	if !strings.HasPrefix(string(key.Id()), "SHA256:") {
		t.Errorf("Id() = %q, want a SHA256 fingerprint", key.Id())
	}
}

// A passphrase-protected key cannot be read, but it must not stop the rest of
// the directory being ingested.
func TestFileConnectionSkipsEncryptedEd25519Key(t *testing.T) {
	dir := t.TempDir()
	edtestCopyFixture(t, dir, "ed25519-encrypted", "id_ed25519_locked")
	edtestCopyFixture(t, dir, "ed25519.pub", "id_ed25519.pub")

	keys, _ := fetchAll(&FileConnection{Type: "FileConnection", Path: dir})

	if len(keys) != 1 {
		t.Fatalf("got %d keys, want only the readable one", len(keys))
	}
	found := keys[0].GetNames()
	if !found.Contains("id_ed25519.pub") {
		t.Errorf("key names %v should be the readable key", found.StringArray())
	}
}
