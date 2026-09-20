package command

import (
	"strings"
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
)

// The rendered strings are also what the positional filters match against, so
// their shape is part of the interface, not just cosmetics.
func TestObjectStrings(t *testing.T) {
	key := data.NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
	acct := data.NewSSHAccount("root", "host.example.com", "conn1", nil)

	tests := []struct {
		name   string
		got    string
		prefix string
		needle string
	}{
		{"key", keyString(key, ""), "key ", "AKIAEXAMPLE"},
		{"account", accountString(acct, ""), "account ", "root@host.example.com"},
		{"connection", connectionString("file:///tmp", ""), "connection ", "file:///tmp"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.HasPrefix(tc.got, tc.prefix) {
				t.Errorf("%q should start with %q", tc.got, tc.prefix)
			}
			if !strings.Contains(tc.got, tc.needle) {
				t.Errorf("%q should mention %q", tc.got, tc.needle)
			}
		})
	}
}

func TestObjectStringsHonourThePrefix(t *testing.T) {
	acct := data.NewSSHAccount("root", "host.example.com", "conn1", nil)

	if got := accountString(acct, "  "); !strings.HasPrefix(got, "  account ") {
		t.Errorf("%q should be indented by the given prefix", got)
	}
}

// A filter built from the rendered form of an object has to match that object,
// which is what makes `locksmith list <substring>` work.
func TestRenderedStringsAreFilterable(t *testing.T) {
	key := data.NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
	filter := buildFilter([]string{"AKIAEXAMPLE"})

	if !filter(keyString(key, "")) {
		t.Error("a filter on the key ID should match the key's rendered form")
	}
	if filter(keyString(data.NewAwsKey("AKIAOTHER", time.Time{}, true, "dev"), "")) {
		t.Error("the filter should not match an unrelated key")
	}
}

func TestOutputKeysForDescribesEveryBinding(t *testing.T) {
	ml := lib.MainLibrary{Path: t.TempDir()}
	keys := ml.Keys()

	key := data.NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
	if err := keys.Store(key); err != nil {
		t.Fatalf("storing key: %v", err)
	}

	acct := data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS},
		{KeyID: "missing-key", Location: data.AUTHORIZED_KEYS},
	})

	// A binding whose key is gone must not stop the listing.
	saved := output.Level
	output.Level = output.SilentLevel
	defer func() { output.Level = saved }()

	outputKeysFor(acct, keys)
}

func TestPrintersSurviveAnEmptyRepository(t *testing.T) {
	ml := lib.MainLibrary{Path: t.TempDir()}

	saved := output.Level
	output.Level = output.SilentLevel
	defer func() { output.Level = saved }()

	printConnections(ml.Connections(), AcceptAll)
	printAccounts(ml.Accounts(), AcceptAll, ml)
	printKeys(ml.Keys(), ml.Accounts(), ml.Policies(), map[data.ID][]data.ID{}, AcceptAll)
	showPendingChanges(ml.Changes(), ml.Keys(), ml.Accounts(), AcceptAll)
}

func TestShowPendingChangesWithAKnownAccount(t *testing.T) {
	ml := lib.MainLibrary{Path: t.TempDir()}

	key := data.NewAwsKey("AKIADEAD", time.Time{}, true, "old")
	key.Expire()
	if err := ml.Keys().Store(key); err != nil {
		t.Fatalf("storing key: %v", err)
	}

	acct := data.NewSSHAccount("root", "host.example.com", "conn1", nil)
	if err := ml.Accounts().Store(acct); err != nil {
		t.Fatalf("storing account: %v", err)
	}

	if err := ml.Changes().Store(data.Change{
		Type:    "Change",
		Account: acct.Id(),
		Remove:  []data.KeyBindingImpl{{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}},
	}); err != nil {
		t.Fatalf("storing change: %v", err)
	}

	saved := output.Level
	output.Level = output.VerboseLevel
	defer func() { output.Level = saved }()

	// Exercises the account lookup and the per-binding rendering.
	showPendingChanges(ml.Changes(), ml.Keys(), ml.Accounts(), AcceptAll)
}
