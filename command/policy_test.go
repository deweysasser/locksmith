package command

import (
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/history"
	"github.com/deweysasser/locksmith/lib"
)

type policyFixture struct {
	ml  lib.MainLibrary
	log *history.Log
}

func newPolicyFixture(t *testing.T) *policyFixture {
	t.Helper()
	repo := t.TempDir()
	return &policyFixture{
		ml:  lib.MainLibrary{Path: repo},
		log: history.Open(repo, "test"),
	}
}

func (f *policyFixture) storeKey(t *testing.T, id string) data.Key {
	t.Helper()
	k := data.NewAwsKey(id, time.Time{}, true, id)
	if err := f.ml.Keys().Store(k); err != nil {
		t.Fatalf("storing key: %v", err)
	}
	return k
}

func (f *policyFixture) policyFor(id data.ID) (data.KeyPolicy, bool) {
	p, err := f.ml.Policies().Fetch(id)
	return p, err == nil
}

// Expiring a key records intent without touching the key record. The key is an
// observation, re-merged from reality on every fetch; intent is not.
func TestExpireWritesPolicyAndLeavesTheKeyAlone(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)
	f.storeKey(t, "AKIADOOMED")

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"}), ""); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}

	p, ok := f.policyFor("AKIADOOMED")
	if !ok {
		t.Fatal("no policy was recorded")
	}
	if p.Disposition != data.DispositionRemove {
		t.Errorf("disposition = %q, want remove", p.Disposition)
	}

	stored, err := f.ml.Keys().Fetch("AKIADOOMED")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if stored.IsDeprecated() {
		t.Error("the key record was mutated; intent belongs in the policy, not on the key")
	}
}

// The thing the old design could not do at any price: Expire only ever set the
// flag, Merge only ever ORed it on, and no command undid it.
func TestUnexpireRevokesIntent(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)
	f.storeKey(t, "AKIADOOMED")

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"}), ""); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}
	if _, ok := f.policyFor("AKIADOOMED"); !ok {
		t.Fatal("no policy to revoke")
	}

	if err := clearPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"})); err != nil {
		t.Fatalf("clearPolicy: %v", err)
	}

	if _, ok := f.policyFor("AKIADOOMED"); ok {
		t.Error("the policy survived unexpire")
	}
}

func TestExpireWithReplacement(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)
	f.storeKey(t, "AKIAOLD")
	f.storeKey(t, "AKIANEW")

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIAOLD"}), "AKIANEW"); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}

	p, ok := f.policyFor("AKIAOLD")
	if !ok {
		t.Fatal("no policy recorded")
	}
	if p.Disposition != data.DispositionReplace {
		t.Errorf("disposition = %q, want replace", p.Disposition)
	}
	if p.Replacement != "AKIANEW" {
		t.Errorf("replacement = %q, want AKIANEW", p.Replacement)
	}
}

// Replacing a key with itself would make plan remove and re-add the same
// binding forever.
func TestExpireRefusesToReplaceAKeyWithItself(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)
	f.storeKey(t, "AKIASAME")

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIASAME"}), "AKIASAME"); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}

	if _, ok := f.policyFor("AKIASAME"); ok {
		t.Error("a self-replacement policy was recorded")
	}
}

// A repository written by an older locksmith carries the flag on the key
// record. It must keep working without a migration step.
func TestLegacyExpiryFlagStillPlans(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	legacy := data.NewAwsKey("AKIALEGACY", time.Time{}, true, "old")
	legacy.Expire()
	if err := f.ml.Keys().Store(legacy); err != nil {
		t.Fatalf("Store: %v", err)
	}

	p, found := effectivePolicy(f.ml.Policies(), legacy)
	if !found {
		t.Fatal("a legacy deprecated key produced no effective policy")
	}
	if p.Disposition != data.DispositionRemove {
		t.Errorf("disposition = %q, want remove", p.Disposition)
	}
}

// ...and unexpire must be able to undo it, or an old repository has a key that
// can never be un-expired.
func TestUnexpireClearsTheLegacyFlag(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	legacy := data.NewAwsKey("AKIALEGACY", time.Time{}, true, "old")
	legacy.Expire()
	if err := f.ml.Keys().Store(legacy); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := clearPolicy(&f.ml, f.log, buildFilter([]string{"AKIALEGACY"})); err != nil {
		t.Fatalf("clearPolicy: %v", err)
	}

	stored, err := f.ml.Keys().Fetch("AKIALEGACY")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if stored.IsDeprecated() {
		t.Error("the legacy flag survived unexpire")
	}
	if _, found := effectivePolicy(f.ml.Policies(), stored); found {
		t.Error("the key still has an effective policy after unexpire")
	}
}

// Expiring a key clears any legacy flag, so a touched key has one source of
// truth rather than two that can disagree.
func TestExpireClearsTheLegacyFlag(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	legacy := data.NewAwsKey("AKIALEGACY", time.Time{}, true, "old")
	legacy.Expire()
	if err := f.ml.Keys().Store(legacy); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIALEGACY"}), ""); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}

	stored, err := f.ml.Keys().Fetch("AKIALEGACY")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if stored.IsDeprecated() {
		t.Error("the legacy flag was left set alongside the new policy")
	}
}

// plan is a diff, not an accumulator: a change that is no longer warranted must
// go. Previously plan only ever wrote, so a change outlived the condition that
// produced it and apply kept reapplying it.
func TestPlanDeletesAChangeThatIsNoLongerWarranted(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	key := f.storeKey(t, "AKIADOOMED")
	acct := data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS},
	})
	if err := f.ml.Accounts().Store(acct); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"}), ""); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}
	calculateChanges(f.ml.Accounts(), f.ml.Keys(), f.ml.Changes(), f.ml.Policies(), AcceptAll)

	if _, err := f.ml.Changes().Fetch(acct.Id()); err != nil {
		t.Fatalf("expected a change after expiring: %v", err)
	}

	// The operator changes their mind.
	if err := clearPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"})); err != nil {
		t.Fatalf("clearPolicy: %v", err)
	}
	calculateChanges(f.ml.Accounts(), f.ml.Keys(), f.ml.Changes(), f.ml.Policies(), AcceptAll)

	if _, err := f.ml.Changes().Fetch(acct.Id()); err == nil {
		t.Error("the stale change survived re-planning after the policy was revoked")
	}
}

// `add` writes a change directly. plan derives its own and must not overwrite
// one the operator just asked for -- and plan is exactly the command they run
// next to inspect it.
func TestPlanLeavesAManualChangeAlone(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	doomed := f.storeKey(t, "AKIADOOMED")
	wanted := f.storeKey(t, "AKIAWANTED")

	acct := data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: doomed.Id(), Location: data.AUTHORIZED_KEYS},
	})
	if err := f.ml.Accounts().Store(acct); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// As `add` would write it.
	if err := f.ml.Changes().Store(data.Change{
		Type:      "Change",
		Manual:    true,
		Account:   acct.Id(),
		ManualAdd: []data.KeyBindingImpl{{KeyID: wanted.Id(), Location: data.AUTHORIZED_KEYS}},
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"}), ""); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}
	calculateChanges(f.ml.Accounts(), f.ml.Keys(), f.ml.Changes(), f.ml.Policies(), AcceptAll)

	got, err := f.ml.Changes().Fetch(acct.Id())
	if err != nil {
		t.Fatalf("the manual change disappeared: %v", err)
	}
	if !got.Manual {
		t.Error("plan overwrote the manual change with a derived one")
	}
	adds := got.Additions()
	if len(adds) != 1 || adds[0].KeyID != wanted.Id() {
		t.Errorf("additions = %v, want the manually added key", adds)
	}
}

// ...but plan still owns the other half of that change. Bailing out of the
// account because an `add` was pending meant the expired key's removal was
// silently discarded -- the one thing a revocation tool must never do.
func TestPlanStillRemovesExpiredKeysAlongsideAManualChange(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	doomed := f.storeKey(t, "AKIADOOMED")
	wanted := f.storeKey(t, "AKIAWANTED")

	acct := data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: doomed.Id(), Location: data.AUTHORIZED_KEYS},
	})
	if err := f.ml.Accounts().Store(acct); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := f.ml.Changes().Store(data.Change{
		Type:      "Change",
		Manual:    true,
		Account:   acct.Id(),
		ManualAdd: []data.KeyBindingImpl{{KeyID: wanted.Id(), Location: data.AUTHORIZED_KEYS}},
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"}), ""); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}
	calculateChanges(f.ml.Accounts(), f.ml.Keys(), f.ml.Changes(), f.ml.Policies(), AcceptAll)

	got, err := f.ml.Changes().Fetch(acct.Id())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got.Remove) != 1 || got.Remove[0].KeyID != doomed.Id() {
		t.Errorf("removals = %v, want the expired key -- it was discarded because an add was pending", got.Remove)
	}
	if adds := got.Additions(); len(adds) != 1 || adds[0].KeyID != wanted.Id() {
		t.Errorf("additions = %v, want the manually added key kept", adds)
	}
}

// A change written by a locksmith that kept both kinds of addition in Add must
// keep working: its additions are the operator's, and re-planning must not
// drop them.
func TestPlanCarriesForwardALegacyManualChange(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	doomed := f.storeKey(t, "AKIADOOMED")
	wanted := f.storeKey(t, "AKIAWANTED")

	acct := data.NewSSHAccount("root", "host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: doomed.Id(), Location: data.AUTHORIZED_KEYS},
	})
	if err := f.ml.Accounts().Store(acct); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// The old on-disk shape: Manual, with the addition in Add.
	if err := f.ml.Changes().Store(data.Change{
		Type:    "Change",
		Manual:  true,
		Account: acct.Id(),
		Add:     []data.KeyBindingImpl{{KeyID: wanted.Id(), Location: data.AUTHORIZED_KEYS}},
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"}), ""); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}
	calculateChanges(f.ml.Accounts(), f.ml.Keys(), f.ml.Changes(), f.ml.Policies(), AcceptAll)

	got, err := f.ml.Changes().Fetch(acct.Id())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if adds := got.Additions(); len(adds) != 1 || adds[0].KeyID != wanted.Id() {
		t.Errorf("additions = %v, want the legacy manual addition preserved", adds)
	}
	if len(got.Remove) != 1 {
		t.Errorf("removals = %v, want the expired key", got.Remove)
	}
}

// A policy is stored under the key ID that was current when `expire` ran. A
// later fetch can learn the key's real public material and change its primary
// ID -- and the policy must still be found, or the repository says the key is
// expired while the fleet goes on honouring it.
func TestEffectivePolicyFindsAPolicyUnderASecondaryIdentifier(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	key := data.NewSSHKeyFromFingerprint("do-key", time.Time{},
		"SHA256:primary", "1e:5f:aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77")

	ids := key.Identifiers()
	if len(ids) < 2 {
		t.Fatalf("need a key with a secondary identifier, got %v", ids)
	}

	// Expire recorded the policy under what was then the primary ID; something
	// later promoted a different identifier to the front.
	if err := f.ml.Policies().Store(data.NewRemovePolicy(ids[len(ids)-1])); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if _, found := effectivePolicy(f.ml.Policies(), key); !found {
		t.Errorf("policy stored under %s was orphaned; key identifiers are %v", ids[len(ids)-1], ids)
	}
}

// A key on its way off systems must never be handed to `add`: binding it
// somewhere new would have the next plan remove it again immediately.
func TestGetKeyIdsSkipsKeysUnderRemovalPolicy(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	f.storeKey(t, "AKIALIVE")
	f.storeKey(t, "AKIADOOMED")

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIADOOMED"}), ""); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}

	got := getKeyIds(f.ml.Keys(), f.ml.Policies(), keyFilter(AcceptAll))

	if len(got) != 1 || got[0] != "AKIALIVE" {
		t.Errorf("got %v, want only the live key", got)
	}
}

// plan's replace path copies the binding and swaps the key, so the
// restrictions the old key was found under travel to its replacement. If they
// did not, rotating a confined key would silently grant its replacement more
// privilege than the key it replaced.
func TestPlanCarriesRestrictionsOntoTheReplacement(t *testing.T) {
	silence(t)
	f := newPolicyFixture(t)

	old := f.storeKey(t, "AKIAOLD")
	replacement := f.storeKey(t, "AKIANEW")

	const opts = `command="/usr/bin/rrsync -ro /srv",restrict`
	acct := data.NewSSHAccount("backup", "backup@host.example.com", "conn1", []data.KeyBindingImpl{
		{KeyID: old.Id(), Location: data.AUTHORIZED_KEYS, Options: opts},
	})
	if err := f.ml.Accounts().Store(acct); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := setPolicy(&f.ml, f.log, buildFilter([]string{"AKIAOLD"}), "AKIANEW"); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}
	calculateChanges(f.ml.Accounts(), f.ml.Keys(), f.ml.Changes(), f.ml.Policies(), AcceptAll)

	change, err := f.ml.Changes().Fetch(acct.Id())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	adds := change.Additions()
	if len(adds) != 1 {
		t.Fatalf("additions = %v, want the replacement", adds)
	}
	if adds[0].KeyID != replacement.Id() {
		t.Fatalf("added %s, want %s", adds[0].KeyID, replacement.Id())
	}
	if adds[0].Options != opts {
		t.Errorf("the replacement was planned without the original restrictions.\nwant: %q\ngot:  %q", opts, adds[0].Options)
	}
}
