package command

import (
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
)

// planFixture is a throwaway repository with its four libraries.
type planFixture struct {
	accounts lib.AccountLibrary
	keys     lib.KeyLibrary
	changes  lib.ChangeLibrary
	policies lib.PolicyLibrary
}

func newPlanFixture(t *testing.T) *planFixture {
	t.Helper()
	ml := lib.MainLibrary{Path: t.TempDir()}
	return &planFixture{
		accounts: ml.Accounts(),
		keys:     ml.Keys(),
		changes:  ml.Changes(),
		policies: ml.Policies(),
	}
}

func (f *planFixture) storeKey(t *testing.T, k data.Key) data.Key {
	t.Helper()
	if err := f.keys.Store(k); err != nil {
		t.Fatalf("storing key: %v", err)
	}
	return k
}

func (f *planFixture) storeAccount(t *testing.T, a data.Account) data.Account {
	t.Helper()
	if err := f.accounts.Store(a); err != nil {
		t.Fatalf("storing account: %v", err)
	}
	return a
}

func (f *planFixture) plannedChanges(t *testing.T) []data.Change {
	t.Helper()
	var got []data.Change
	for c := range f.changes.List() {
		got = append(got, c)
	}
	return got
}

func bindingTo(id data.ID) []data.KeyBindingImpl {
	return []data.KeyBindingImpl{{KeyID: id, Location: data.AUTHORIZED_KEYS}}
}

// A key in good standing produces no work.
func TestCalculateChangesLeavesHealthyKeysAlone(t *testing.T) {
	f := newPlanFixture(t)

	key := f.storeKey(t, data.NewAwsKey("AKIALIVE", time.Time{}, true, "prod"))
	f.storeAccount(t, data.NewSSHAccount("root", "host.example.com", "conn1", bindingTo(key.Id())))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	if got := f.plannedChanges(t); len(got) != 0 {
		t.Errorf("planned %d changes for a healthy account, want none: %v", len(got), got)
	}
}

func TestCalculateChangesRemovesDeprecatedKeys(t *testing.T) {
	f := newPlanFixture(t)

	key := data.NewAwsKey("AKIADEAD", time.Time{}, true, "old")
	key.Expire()
	f.storeKey(t, key)

	acct := f.storeAccount(t, data.NewSSHAccount("root", "host.example.com", "conn1", bindingTo(key.Id())))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	got := f.plannedChanges(t)
	if len(got) != 1 {
		t.Fatalf("planned %d changes, want 1", len(got))
	}
	if got[0].Account != acct.Id() {
		t.Errorf("change is for account %q, want %q", got[0].Account, acct.Id())
	}
	if len(got[0].Remove) != 1 || got[0].Remove[0].KeyID != key.Id() {
		t.Errorf("removals = %v, want the deprecated key", got[0].Remove)
	}
	if len(got[0].Add) != 0 {
		t.Errorf("additions = %v, want none", got[0].Add)
	}
}

func TestCalculateChangesAddsReplacementKeys(t *testing.T) {
	f := newPlanFixture(t)

	replacement := f.storeKey(t, data.NewAwsKey("AKIANEW", time.Time{}, true, "new"))

	old := data.NewAwsKey("AKIAOLD", time.Time{}, true, "old")
	old.Replacement = replacement.Id()
	f.storeKey(t, old)

	f.storeAccount(t, data.NewSSHAccount("root", "host.example.com", "conn1", bindingTo(old.Id())))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	got := f.plannedChanges(t)
	if len(got) != 1 {
		t.Fatalf("planned %d changes, want 1", len(got))
	}
	if len(got[0].Add) != 1 || got[0].Add[0].KeyID != replacement.Id() {
		t.Errorf("additions = %v, want the replacement key", got[0].Add)
	}
}

// Rotation is the whole point: the old key goes and the new one arrives, in one
// change, at the same binding location.
func TestCalculateChangesRotatesAKey(t *testing.T) {
	f := newPlanFixture(t)

	replacement := f.storeKey(t, data.NewAwsKey("AKIANEW", time.Time{}, true, "new"))

	old := data.NewAwsKey("AKIAOLD", time.Time{}, true, "old")
	old.Replacement = replacement.Id()
	old.Expire()
	f.storeKey(t, old)

	f.storeAccount(t, data.NewSSHAccount("root", "host.example.com", "conn1", bindingTo(old.Id())))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	got := f.plannedChanges(t)
	if len(got) != 1 {
		t.Fatalf("planned %d changes, want 1", len(got))
	}
	if len(got[0].Add) != 1 || got[0].Add[0].KeyID != replacement.Id() {
		t.Errorf("additions = %v, want the replacement key", got[0].Add)
	}
	if len(got[0].Remove) != 1 || got[0].Remove[0].KeyID != old.Id() {
		t.Errorf("removals = %v, want the expired key", got[0].Remove)
	}
	if got[0].Add[0].Location != data.AUTHORIZED_KEYS {
		t.Errorf("the replacement should land where the old key was, got %q", got[0].Add[0].Location)
	}
}

func TestCalculateChangesRespectsTheFilter(t *testing.T) {
	f := newPlanFixture(t)

	key := data.NewAwsKey("AKIADEAD", time.Time{}, true, "old")
	key.Expire()
	f.storeKey(t, key)

	f.storeAccount(t, data.NewSSHAccount("root", "wanted.example.com", "conn1", bindingTo(key.Id())))
	f.storeAccount(t, data.NewSSHAccount("root", "ignored.example.com", "conn1", bindingTo(key.Id())))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, buildFilter([]string{"wanted.example.com"}))

	got := f.plannedChanges(t)
	if len(got) != 1 {
		t.Fatalf("planned %d changes, want 1", len(got))
	}
	if got[0].Account != "root@wanted.example.com" {
		t.Errorf("planned a change for %q, want the filtered account", got[0].Account)
	}
}

// A binding can outlive the key it points at, for instance after `remove`.
// Planning should skip it rather than crash.
func TestCalculateChangesSkipsBindingsWithNoKey(t *testing.T) {
	f := newPlanFixture(t)

	f.storeAccount(t, data.NewSSHAccount("root", "host.example.com", "conn1", bindingTo("vanished-key")))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	if got := f.plannedChanges(t); len(got) != 0 {
		t.Errorf("planned %d changes for a dangling binding, want none", len(got))
	}
}

func TestCalculateChangesWithNoAccounts(t *testing.T) {
	f := newPlanFixture(t)

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	if got := f.plannedChanges(t); len(got) != 0 {
		t.Errorf("planned %d changes against an empty repository, want none", len(got))
	}
}

// Planning twice must not double the pending work.
func TestCalculateChangesIsIdempotent(t *testing.T) {
	f := newPlanFixture(t)

	key := data.NewAwsKey("AKIADEAD", time.Time{}, true, "old")
	key.Expire()
	f.storeKey(t, key)
	f.storeAccount(t, data.NewSSHAccount("root", "host.example.com", "conn1", bindingTo(key.Id())))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)
	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	if got := f.plannedChanges(t); len(got) != 1 {
		t.Errorf("planning twice produced %d changes, want 1", len(got))
	}
}

// One account per change, even when several accounts need the same work.
func TestCalculateChangesOneChangePerAccount(t *testing.T) {
	f := newPlanFixture(t)

	key := data.NewAwsKey("AKIADEAD", time.Time{}, true, "old")
	key.Expire()
	f.storeKey(t, key)

	for _, host := range []string{"a.example.com", "b.example.com", "c.example.com"} {
		f.storeAccount(t, data.NewSSHAccount("root", host, "conn1", bindingTo(key.Id())))
	}

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	got := f.plannedChanges(t)
	if len(got) != 3 {
		t.Fatalf("planned %d changes, want one per account (3)", len(got))
	}

	seen := make(map[data.ID]bool)
	for _, c := range got {
		if seen[c.Account] {
			t.Errorf("account %q got more than one change", c.Account)
		}
		seen[c.Account] = true
	}
}

// An account with several bindings collects all of its work into one change.
func TestCalculateChangesGroupsBindingsPerAccount(t *testing.T) {
	f := newPlanFixture(t)

	first := data.NewAwsKey("AKIADEAD1", time.Time{}, true, "old1")
	first.Expire()
	second := data.NewAwsKey("AKIADEAD2", time.Time{}, true, "old2")
	second.Expire()
	f.storeKey(t, first)
	f.storeKey(t, second)

	f.storeAccount(t, data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: first.Id(), Location: data.AUTHORIZED_KEYS},
		{KeyID: second.Id(), Location: data.AUTHORIZED_KEYS},
	}))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	got := f.plannedChanges(t)
	if len(got) != 1 {
		t.Fatalf("planned %d changes, want 1", len(got))
	}
	if len(got[0].Remove) != 2 {
		t.Errorf("removals = %v, want both expired keys in one change", got[0].Remove)
	}
}

func TestNewBindingKeepsLocationAndName(t *testing.T) {
	original := data.KeyBindingImpl{
		KeyID:    "oldkey",
		Location: data.AWS_CREDENTIALS,
		Name:     "deploy",
	}

	got := newBinding(original, "newkey")

	if got.KeyID != "newkey" {
		t.Errorf("KeyID = %q, want newkey", got.KeyID)
	}
	if got.Location != data.AWS_CREDENTIALS {
		t.Errorf("Location = %q, want it carried over", got.Location)
	}
	if got.Name != "deploy" {
		t.Errorf("Name = %q, want it carried over", got.Name)
	}
	if original.KeyID != "oldkey" {
		t.Error("newBinding should not modify the binding it was given")
	}
}

// Re-planning after the situation changes must replace the account's pending
// change, not leave a stale one beside it.
func TestCalculateChangesReplacesAStalePlan(t *testing.T) {
	f := newPlanFixture(t)

	first := data.NewAwsKey("AKIADEAD1", time.Time{}, true, "old1")
	first.Expire()
	f.storeKey(t, first)

	second := data.NewAwsKey("AKIADEAD2", time.Time{}, true, "old2")
	f.storeKey(t, second)

	f.storeAccount(t, data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: first.Id(), Location: data.AUTHORIZED_KEYS},
		{KeyID: second.Id(), Location: data.AUTHORIZED_KEYS},
	}))

	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	// The operator expires the second key and re-plans.
	second.Expire()
	f.storeKey(t, second)
	calculateChanges(f.accounts, f.keys, f.changes, f.policies, AcceptAll)

	got := f.plannedChanges(t)
	if len(got) != 1 {
		t.Fatalf("re-planning produced %d changes for one account, want 1: %v", len(got), got)
	}
	if len(got[0].Remove) != 2 {
		t.Errorf("removals = %v, want both expired keys", got[0].Remove)
	}
}
