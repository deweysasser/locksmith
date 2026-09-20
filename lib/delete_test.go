package lib

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
)

func TestLibraryDelete(t *testing.T) {
	dir := t.TempDir()
	lib := new(library)
	lib.Init(dir, nil, createEntry)

	e := entry{"id1", "testing1"}
	if err := lib.Store(&e); err != nil {
		t.Fatalf("Store: %v", err)
	}

	path := filepath.Join(dir, "id1.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}

	if err := lib.Delete("id1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("the stored file should be gone after Delete")
	}
	if _, err := lib.Fetch("id1"); err == nil {
		t.Error("a deleted object should no longer be fetchable")
	}
}

// Delete has to clear the in-memory cache too, or the object stays visible for
// the rest of the run.
func TestLibraryDeleteEvictsFromCache(t *testing.T) {
	dir := t.TempDir()
	lib := new(library)
	lib.Init(dir, nil, createEntry)

	e := entry{"id1", "testing1"}
	if err := lib.Store(&e); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Warm the cache.
	if _, err := lib.Fetch("id1"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if err := lib.Delete("id1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := lib.Fetch("id1"); err == nil {
		t.Error("a deleted object should not still be served from the cache")
	}
}

func TestLibraryDeleteMissingObject(t *testing.T) {
	lib := new(library)
	lib.Init(t.TempDir(), nil, createEntry)

	if err := lib.Delete("never-stored"); err == nil {
		t.Error("deleting an object that was never stored should report an error")
	}
}

func TestLibraryDeleteObject(t *testing.T) {
	dir := t.TempDir()
	lib := new(library)
	lib.Init(dir, nil, createEntry)

	e := entry{"id1", "testing1"}
	if err := lib.Store(&e); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := lib.DeleteObject(&e); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "id1.json")); err == nil {
		t.Error("the stored file should be gone after DeleteObject")
	}
}

// An object stored under several identifiers has to be evicted under all of
// them, not just the one it was deleted by.
func TestLibraryDeleteEvictsEverySecondaryId(t *testing.T) {
	dir := t.TempDir()
	lib := library{Path: dir, deserializer: deserializeMultiID}

	m := multiID{[]data.ID{"id1", "id2"}}
	if err := lib.Store(&m); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if _, err := lib.Fetch("id2"); err != nil {
		t.Fatalf("expected to find the object by its secondary ID: %v", err)
	}

	if err := lib.Delete("id1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := lib.Fetch("id2"); err == nil {
		t.Error("deleting by the primary ID should also evict the secondary ID")
	}
}

func TestLibraryListOnMissingDirectory(t *testing.T) {
	lib := new(library)
	lib.Init(filepath.Join(t.TempDir(), "never-created"), nil, createEntry)

	count := 0
	for range lib.List() {
		count++
	}

	if count != 0 {
		t.Errorf("listing a directory that does not exist gave %d objects, want 0", count)
	}
}

func TestLibraryStoreCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deeply", "nested")
	lib := new(library)
	lib.Init(dir, nil, createEntry)

	e := entry{"id1", "testing1"}
	if err := lib.Store(&e); err != nil {
		t.Fatalf("Store should create its directory: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "id1.json")); err != nil {
		t.Errorf("expected the object to be stored: %v", err)
	}
}

// Identifiers become filenames, so anything that is not a word character is
// flattened.
func TestSanitize(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"simple", "simple"},
		{"root@host.example.com", "root_host_example_com"},
		{"SHA256:abc/def+ghi", "SHA256_abc_def_ghi"},
		{"arn:aws:iam::123456789012:user/someone", "arn_aws_iam_123456789012_user_someone"},
		{"../../escape", "_escape"},
	}

	for _, tc := range tests {
		if got := sanitize(tc.in); got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A key ID contains slashes and colons; the file it lands in must stay inside
// the library directory.
func TestPathOfIdStaysInsideTheLibrary(t *testing.T) {
	lib := new(library)
	lib.Init("/tmp/lib", nil, createEntry)

	got := lib.pathOfId("../../etc/passwd")

	if want := "/tmp/lib/_etc_passwd.json"; got != want {
		t.Errorf("pathOfId = %q, want %q", got, want)
	}
}

func TestHashIsStable(t *testing.T) {
	if hashString("abc") != hashString("abc") {
		t.Error("hashing the same string twice should give the same result")
	}
	if hashString("abc") == hashString("abd") {
		t.Error("different strings should hash differently")
	}
}

func TestLibraryIdFallsBackToStringer(t *testing.T) {
	lib := new(library)
	lib.Init(t.TempDir(), nil, nil)

	// An SSH account is an Ider, so its own ID wins.
	acct := data.NewSSHAccount("root", "host.example.com", "conn1", nil)
	if got := lib.Id(acct); got != "root@host.example.com" {
		t.Errorf("Id() = %q, want the account's own ID", got)
	}
}

func TestLibraryIdUsesIdString(t *testing.T) {
	lib := new(library)
	lib.Init(t.TempDir(), nil, nil)

	if got := lib.Id(&entry{"id1", "content"}); got != "id1" {
		t.Errorf("Id() = %q, want id1", got)
	}
}

func TestMainLibraryUsesSeparateSubdirectories(t *testing.T) {
	dir := t.TempDir()
	ml := MainLibrary{Path: dir}

	key := data.NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
	acct := data.NewSSHAccount("root", "host.example.com", "conn1", nil)

	if err := ml.Keys().Store(key); err != nil {
		t.Fatalf("storing key: %v", err)
	}
	if err := ml.Accounts().Store(acct); err != nil {
		t.Fatalf("storing account: %v", err)
	}

	for _, sub := range []string{"keys", "accounts"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Errorf("expected a %s/ subdirectory: %v", sub, err)
		}
	}
}
