package connection

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/deweysasser/locksmith/data"
)

func TestBasename(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"rsa.pub", "rsa.pub"},
		{"dir/rsa.pub", "rsa.pub"},
		{"a/b/c/rsa.pub", "rsa.pub"},
		{"/home/someone/.ssh/rsa.pub", "rsa.pub"},
		// A key sitting at the filesystem root still has a name of its own.
		{"/rsa.pub", "rsa.pub"},
		{"/", ""},
		{"", ""},
		{"trailing/", ""},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := basename(tc.path); got != tc.want {
				t.Errorf("basename(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestMatches(t *testing.T) {
	tests := []struct {
		re   string
		name string
		want bool
	}{
		{"~$", "id_rsa~", true},
		{"~$", "id_rsa", false},
		{"^#.*", "#id_rsa#", true},
		{"^#.*", "id_rsa", false},
		// An unparseable expression matches nothing rather than exploding.
		{"[", "anything", false},
	}

	for _, tc := range tests {
		if got := matches(tc.re, tc.name); got != tc.want {
			t.Errorf("matches(%q, %q) = %v, want %v", tc.re, tc.name, got, tc.want)
		}
	}
}

// stat returns the FileInfo for a file created inside the test's temp dir.
func stat(t *testing.T, name string) os.FileInfo {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := ioutil.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info
}

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"id_rsa.pub", false},
		{"authorized_keys", false},
		{"id_rsa.pub~", true},
		{"#id_rsa.pub#", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipFile(stat(t, tc.name)); got != tc.want {
				t.Errorf("shouldSkipFile(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestFileConnectionString(t *testing.T) {
	c := &FileConnection{Type: "FileConnection", Path: "/home/someone/.ssh"}

	if got, want := c.String(), "file:///home/someone/.ssh"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestFileConnectionIdIsDerivedFromPath(t *testing.T) {
	a := &FileConnection{Type: "FileConnection", Path: "/home/someone/.ssh"}
	b := &FileConnection{Type: "FileConnection", Path: "/home/someone/.ssh"}
	c := &FileConnection{Type: "FileConnection", Path: "/elsewhere"}

	if a.Id() != b.Id() {
		t.Error("the same path should always produce the same connection ID")
	}
	if a.Id() == c.Id() {
		t.Error("different paths should produce different connection IDs")
	}
	if a.Id() != data.IdFromString("/home/someone/.ssh") {
		t.Errorf("Id() = %q, want the hash of the path", a.Id())
	}
}

// fetchAll drains both channels a connection returns.
func fetchAll(c Connection) ([]data.Key, []data.Account) {
	keyChan, acctChan := c.Fetch()

	var keys []data.Key
	var accounts []data.Account

	done := make(chan struct{})
	go func() {
		defer close(done)
		for a := range acctChan {
			accounts = append(accounts, a)
		}
	}()

	for k := range keyChan {
		keys = append(keys, k)
	}
	<-done

	return keys, accounts
}

func TestFileConnectionFetchDirectory(t *testing.T) {
	c := &FileConnection{Type: "FileConnection", Path: "../data/test-data/public-keys"}

	keys, accounts := fetchAll(c)

	if len(keys) == 0 {
		t.Fatal("fetching a directory of public keys produced no keys")
	}
	if len(accounts) != 0 {
		t.Errorf("a file connection should discover no accounts, got %d", len(accounts))
	}

	// Every key should be named after the file it came from, not the full path.
	for _, k := range keys {
		names := k.GetNames()
		for _, n := range names.StringArray() {
			if filepath.Base(n) != n {
				t.Errorf("key name %q should be a bare filename", n)
			}
		}
	}
}

func TestFileConnectionFetchSingleFile(t *testing.T) {
	c := &FileConnection{Type: "FileConnection", Path: "../data/test-data/rsa.pub"}

	keys, _ := fetchAll(c)

	if len(keys) != 1 {
		t.Fatalf("got %d keys from a single public key file, want 1", len(keys))
	}
	names := keys[0].GetNames()
	if !names.Contains("rsa.pub") {
		t.Errorf("key names %v should include the file name", names.StringArray())
	}
}

func TestFileConnectionFetchCredentials(t *testing.T) {
	c := &FileConnection{Type: "FileConnection", Path: "../data/test-data/credentials"}

	keys, _ := fetchAll(c)

	var ids []string
	for _, k := range keys {
		ids = append(ids, string(k.Id()))
	}
	sort.Strings(ids)

	want := []string{"AKIAI44QH8DHBEXAMPLE", "AKIAIOSFODNN7EXAMPLE"}
	if len(ids) != len(want) {
		t.Fatalf("got %d keys %v, want %d %v", len(ids), ids, len(want), want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("key[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestFileConnectionFetchMissingPath(t *testing.T) {
	c := &FileConnection{Type: "FileConnection", Path: filepath.Join(t.TempDir(), "does-not-exist")}

	keys, accounts := fetchAll(c)

	if len(keys) != 0 || len(accounts) != 0 {
		t.Errorf("a missing path should yield nothing, got %d keys and %d accounts", len(keys), len(accounts))
	}
}

func TestFileConnectionSkipsBackupFiles(t *testing.T) {
	dir := t.TempDir()

	pub, err := ioutil.ReadFile("../data/test-data/rsa.pub")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	for _, name := range []string{"good.pub", "backup.pub~", "#interrupted.pub#"} {
		if err := ioutil.WriteFile(filepath.Join(dir, name), pub, 0600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	keys, _ := fetchAll(&FileConnection{Type: "FileConnection", Path: dir})

	if len(keys) != 1 {
		t.Fatalf("got %d keys, want 1 (backup and autosave files should be skipped)", len(keys))
	}
	names := keys[0].GetNames()
	if !names.Contains("good.pub") {
		t.Errorf("key names %v should be the non-backup file", names.StringArray())
	}
}

// known_hosts entries are host keys, not user keys, and should never be
// ingested as credentials.
func TestFileConnectionSkipsKnownHosts(t *testing.T) {
	dir := t.TempDir()

	pub, err := ioutil.ReadFile("../data/test-data/rsa.pub")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "known_hosts"), pub, 0600); err != nil {
		t.Fatalf("writing known_hosts: %v", err)
	}

	keys, _ := fetchAll(&FileConnection{Type: "FileConnection", Path: dir})

	if len(keys) != 0 {
		t.Errorf("got %d keys from a known_hosts file, want 0", len(keys))
	}
}

func TestFileConnectionRecursesIntoSubdirectories(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pub, err := ioutil.ReadFile("../data/test-data/rsa.pub")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(nested, "deep.pub"), pub, 0600); err != nil {
		t.Fatalf("writing key: %v", err)
	}

	keys, _ := fetchAll(&FileConnection{Type: "FileConnection", Path: dir})

	if len(keys) != 1 {
		t.Fatalf("got %d keys, want 1 from a nested directory", len(keys))
	}
}

// A directory of keys is the common case for `locksmith connect ~/.ssh`, and an
// authorized_keys file in it holds many entries. Ingesting only the first means
// the operator is told a key is gone when it is still authorized -- the worst
// failure this tool has.
func TestFileConnectionReadsEveryKeyInAMultiKeyFile(t *testing.T) {
	dir := t.TempDir()

	body, err := ioutil.ReadFile("../data/test-data/authorized_keys")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "authorized_keys"), body, 0600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	keys, _ := fetchAll(&FileConnection{Type: "FileConnection", Path: dir})

	if len(keys) < 5 {
		t.Fatalf("fetched %d keys from a multi-key file, want every entry (at least 5)", len(keys))
	}

	seen := make(map[data.ID]bool)
	for _, k := range keys {
		if seen[k.Id()] {
			t.Errorf("key %q was fetched twice", k.Id())
		}
		seen[k.Id()] = true
	}
}
