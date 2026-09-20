package data

import (
	"io/ioutil"
	"testing"
	"time"
)

// An authorized_keys file is the central artifact this tool exists to read, and
// it holds many entries. Taking only the first silently drops the rest: the
// operator is told a key is gone when it is still authorized.
func TestNewKeysReadsEveryEntryInAFile(t *testing.T) {
	body, err := ioutil.ReadFile("test-data/authorized_keys")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	keys := NewKeys(string(body), time.Now(), "authorized_keys")

	if len(keys) < 5 {
		t.Fatalf("read %d keys from the fixture, want every entry (at least 5)", len(keys))
	}

	seen := make(map[ID]bool)
	for _, k := range keys {
		if k.Id() == "" {
			t.Error("a key came back with no identifier")
		}
		if seen[k.Id()] {
			t.Errorf("key %q was returned twice", k.Id())
		}
		seen[k.Id()] = true
	}
}

func TestNewKeysAcrossAlgorithms(t *testing.T) {
	body, err := ioutil.ReadFile("test-data/authorized_keys")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	types := make(map[string]bool)
	for _, k := range NewKeys(string(body), time.Now()) {
		if sk, ok := k.(*SSHKey); ok {
			types[sk.KeyType()] = true
		}
	}

	for _, want := range []string{"ssh-rsa", "ssh-ed25519", "ecdsa-sha2-nistp256"} {
		if !types[want] {
			t.Errorf("no %s key came back from the fixture; got %v", want, types)
		}
	}
}

func TestNewKeysHandlesEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"blank lines only", "\n\n\n", 0},
		{"prose", "this file holds no keys at all\n", 0},
		{"comments only", "# my keys\n# more\n", 0},
		{"one key", testPublicKey + " someone@example.com\n", 1},
		{"no trailing newline", testPublicKey + " someone@example.com", 1},
		{"key surrounded by blanks and comments", "\n# keys\n\n" + testPublicKey + " a@b\n\n", 1},
		{"a PuTTY file is declined whole", "PuTTY-User-Key-File-2: ssh-rsa\nmore\n", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewKeys(tc.content, time.Now()); len(got) != tc.want {
				t.Errorf("NewKeys returned %d keys, want %d", len(got), tc.want)
			}
		})
	}
}

// A private key file is one key, and its body must not be scanned line by line.
func TestNewKeysOnAPrivateKeyFile(t *testing.T) {
	body, err := ioutil.ReadFile("test-data/rsa")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	keys := NewKeys(string(body), time.Now(), "rsa")

	if len(keys) != 1 {
		t.Fatalf("read %d keys from a private key file, want 1", len(keys))
	}
	if sk, ok := keys[0].(*SSHKey); !ok || sk.PublicKey.Key == nil {
		t.Error("a private key should yield its public half")
	}
}
