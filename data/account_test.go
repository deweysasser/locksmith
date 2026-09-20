package data

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
)

func collectBindings(a Account) []KeyBindingImpl {
	var got []KeyBindingImpl
	for b := range a.Bindings() {
		got = append(got, b)
	}
	return got
}

func TestNewSSHAccountStripsUserFromHost(t *testing.T) {
	tests := []struct {
		name     string
		username string
		target   string
		wantHost string
		wantId   ID
	}{
		{"bare hostname", "root", "host.example.com", "host.example.com", "root@host.example.com"},
		{"user@host is reduced to the host", "root", "ubuntu@host.example.com", "host.example.com", "root@host.example.com"},
		{"an empty target yields an empty host", "root", "", "", "root@"},
		{"only the first @ splits", "root", "a@b@c", "b@c", "root@b@c"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := NewSSHAccount(tc.username, tc.target, "conn1", nil)

			if a.Host != tc.wantHost {
				t.Errorf("Host = %q, want %q", a.Host, tc.wantHost)
			}
			if a.Id() != tc.wantId {
				t.Errorf("Id() = %q, want %q", a.Id(), tc.wantId)
			}
			if a.ConnectionID() != "conn1" {
				t.Errorf("ConnectionID() = %q, want conn1", a.ConnectionID())
			}
			if a.Type != "SSHAccount" {
				t.Errorf("Type = %q, want SSHAccount (the persisted type tag)", a.Type)
			}
		})
	}
}

func TestSSHAccountString(t *testing.T) {
	a := NewSSHAccount("root", "host.example.com", "conn1", nil)

	if got, want := a.String(), "SSH root@host.example.com"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestAccountBindings(t *testing.T) {
	bindings := []KeyBindingImpl{
		{KeyID: "key1", Location: AUTHORIZED_KEYS},
		{KeyID: "key2", Location: AUTHORIZED_KEYS},
	}
	a := NewSSHAccount("root", "host", "conn1", bindings)

	got := collectBindings(a)
	if len(got) != 2 {
		t.Fatalf("got %d bindings, want 2", len(got))
	}
}

func TestAccountBindingsOnEmptyAccount(t *testing.T) {
	a := NewSSHAccount("root", "host", "conn1", nil)

	if got := collectBindings(a); len(got) != 0 {
		t.Errorf("got %d bindings from an empty account, want 0", len(got))
	}
}

func TestAddBinding(t *testing.T) {
	a := NewSSHAccount("root", "host", "conn1", nil)
	key := NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")

	a.AddBinding(key, AUTHORIZED_KEYS, "")

	got := collectBindings(a)
	if len(got) != 1 {
		t.Fatalf("got %d bindings, want 1", len(got))
	}
	if got[0].KeyID != key.Id() {
		t.Errorf("binding key = %q, want %q", got[0].KeyID, key.Id())
	}
}

func TestSSHAccountMergeUnionsBindings(t *testing.T) {
	a := NewSSHAccount("root", "host", "conn1", []KeyBindingImpl{
		{KeyID: "key1", Location: AUTHORIZED_KEYS},
	})
	b := NewSSHAccount("root", "host", "conn1", []KeyBindingImpl{
		{KeyID: "key2", Location: AUTHORIZED_KEYS},
	})

	a.Merge(b)

	if got := collectBindings(a); len(got) != 2 {
		t.Errorf("got %d bindings after merge, want 2", len(got))
	}
}

func TestSSHAccountMergeDeduplicates(t *testing.T) {
	binding := KeyBindingImpl{KeyID: "key1", Location: AUTHORIZED_KEYS}
	a := NewSSHAccount("root", "host", "conn1", []KeyBindingImpl{binding})
	b := NewSSHAccount("root", "host", "conn1", []KeyBindingImpl{binding})

	a.Merge(b)

	if got := collectBindings(a); len(got) != 1 {
		t.Errorf("got %d bindings after merging identical bindings, want 1", len(got))
	}
}

// The same key bound in two places is two distinct bindings.
func TestSSHAccountMergeKeepsDistinctLocations(t *testing.T) {
	a := NewSSHAccount("root", "host", "conn1", []KeyBindingImpl{
		{KeyID: "key1", Location: AUTHORIZED_KEYS},
	})
	b := NewSSHAccount("root", "host", "conn1", []KeyBindingImpl{
		{KeyID: "key1", Location: AWS_CREDENTIALS},
	})

	a.Merge(b)

	if got := collectBindings(a); len(got) != 2 {
		t.Errorf("got %d bindings, want 2 (same key, two locations)", len(got))
	}
}

// Accounts are re-merged on every fetch and written back to disk. If the merge
// order were unstable the JSON would churn on every run, which defeats the
// documented promise that the repository is safe to keep in git.
func TestMergeBindingsIsDeterministic(t *testing.T) {
	b1 := []KeyBindingImpl{
		{KeyID: "ccc", Location: AUTHORIZED_KEYS},
		{KeyID: "aaa", Location: AUTHORIZED_KEYS},
	}
	b2 := []KeyBindingImpl{
		{KeyID: "bbb", Location: AUTHORIZED_KEYS},
	}

	first := mergeBindings(b1, b2, nil)
	for i := 0; i < 20; i++ {
		if got := mergeBindings(b1, b2, nil); !reflect.DeepEqual(got, first) {
			t.Fatalf("merge order is unstable:\n run 1: %v\n run %d: %v", first, i+2, got)
		}
	}

	if len(first) != 3 {
		t.Fatalf("got %d bindings, want 3", len(first))
	}
}

func TestMergeBindingsEmptyInputs(t *testing.T) {
	if got := mergeBindings(nil, nil, nil); len(got) != 0 {
		t.Errorf("merging two empty binding sets gave %v, want nothing", got)
	}
}

func TestAWSAccount(t *testing.T) {
	a := NewAWSAccount("123456789012", "conn1", nil, "prod", "")

	if a.Id() != "123456789012" {
		t.Errorf("Id() = %q, want 123456789012", a.Id())
	}
	if got, want := a.String(), "123456789012 (prod)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestAWSAccountWithoutAliases(t *testing.T) {
	a := NewAWSAccount("123456789012", "conn1", nil)

	if got, want := a.String(), "aws 123456789012"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestAWSAccountMerge(t *testing.T) {
	a := NewAWSAccount("123456789012", "conn1", []KeyBindingImpl{{KeyID: "key1"}}, "prod")
	b := NewAWSAccount("123456789012", "conn1", []KeyBindingImpl{{KeyID: "key2"}}, "prod")

	a.Merge(b)

	if got := collectBindings(a); len(got) != 2 {
		t.Errorf("got %d bindings after merge, want 2", len(got))
	}
}

func TestNewIAMAccount(t *testing.T) {
	created := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)
	user := &iam.User{
		Arn:        aws.String("arn:aws:iam::123456789012:user/someone"),
		UserName:   aws.String("someone"),
		CreateDate: aws.Time(created),
	}

	a := NewIAMAccount(user, "conn1")

	if a.Id() != "arn:aws:iam::123456789012:user/someone" {
		t.Errorf("Id() = %q, want the ARN", a.Id())
	}
	if ids := a.Identifiers(); len(ids) != 1 || ids[0] != a.Id() {
		t.Errorf("Identifiers() = %v, want [%s]", ids, a.Id())
	}
	if a.Username != "someone" {
		t.Errorf("Username = %q, want someone", a.Username)
	}
	if !a.CreateDate.Equal(created) {
		t.Errorf("CreateDate = %s, want %s", a.CreateDate, created)
	}
	if got := a.String(); got != "arn:aws:iam::123456789012:user/someone" {
		t.Errorf("String() = %q, want the ARN", got)
	}
	if got := collectBindings(a); len(got) != 0 {
		t.Errorf("a fresh IAM account should have no bindings, got %v", got)
	}
}

func TestNewIAMAccountFromKey(t *testing.T) {
	user := &iam.User{
		Arn:        aws.String("arn:aws:iam::123456789012:user/someone"),
		UserName:   aws.String("someone"),
		CreateDate: aws.Time(time.Time{}),
	}
	meta := &iam.AccessKeyMetadata{AccessKeyId: aws.String("AKIAEXAMPLE")}

	a := NewIAMAccountFromKey(meta, user, "conn1")

	got := collectBindings(a)
	if len(got) != 1 {
		t.Fatalf("got %d bindings, want 1", len(got))
	}
	if got[0].KeyID != "AKIAEXAMPLE" {
		t.Errorf("binding key = %q, want AKIAEXAMPLE", got[0].KeyID)
	}
}

func TestIAMAccountMergeFillsBlanks(t *testing.T) {
	created := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)

	// An account discovered by access key has no create date of its own.
	sparse := &AWSIamAccount{
		accountImpl: accountImpl{Type: "AWSIamAccount", Connection: "conn1"},
		Arn:         "arn:aws:iam::123456789012:user/someone",
	}
	full := NewIAMAccount(&iam.User{
		Arn:        aws.String("arn:aws:iam::123456789012:user/someone"),
		UserName:   aws.String("someone"),
		CreateDate: aws.Time(created),
	}, "conn1")

	sparse.Merge(full)

	if !sparse.CreateDate.Equal(created) {
		t.Errorf("CreateDate = %s, want it filled in from the merge (%s)", sparse.CreateDate, created)
	}
}

func TestNewAWSInstanceAccount(t *testing.T) {
	instance := &ec2.Instance{
		InstanceId:    aws.String("i-0123456789abcdef0"),
		PublicDnsName: aws.String("ec2-1-2-3-4.compute.amazonaws.com"),
		Tags: []*ec2.Tag{
			{Key: aws.String("Environment"), Value: aws.String("prod")},
			{Key: aws.String("Name"), Value: aws.String("web-1")},
		},
	}

	a := NewAWSInstanceAccount(instance, "conn1", nil)

	if a.Id() != "i-0123456789abcdef0" {
		t.Errorf("Id() = %q, want the instance ID", a.Id())
	}
	if a.NameTag != "web-1" {
		t.Errorf("NameTag = %q, want web-1", a.NameTag)
	}
	if got, want := a.String(), "i-0123456789abcdef0 (web-1, ec2-1-2-3-4.compute.amazonaws.com)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestAWSInstanceAccountStringWithoutTagsOrDNS(t *testing.T) {
	instance := &ec2.Instance{
		InstanceId:    aws.String("i-0123456789abcdef0"),
		PublicDnsName: aws.String(""),
	}

	a := NewAWSInstanceAccount(instance, "conn1", nil)

	if got, want := a.String(), "i-0123456789abcdef0"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// A stopped instance gets a new public DNS name when it restarts, so the
// freshly fetched value wins.
func TestAWSInstanceAccountMergeTakesNewDNS(t *testing.T) {
	old := NewAWSInstanceAccount(&ec2.Instance{
		InstanceId:    aws.String("i-0123456789abcdef0"),
		PublicDnsName: aws.String("old.example.com"),
	}, "conn1", nil)
	fresh := NewAWSInstanceAccount(&ec2.Instance{
		InstanceId:    aws.String("i-0123456789abcdef0"),
		PublicDnsName: aws.String("new.example.com"),
	}, "conn1", nil)

	old.Merge(fresh)

	if old.PublicDNS != "new.example.com" {
		t.Errorf("PublicDNS = %q, want the freshly fetched name", old.PublicDNS)
	}
}

// --- observation authority -------------------------------------------------
//
// mergeBindings with a non-nil authority list is the only code in the tree that
// *deletes* recorded inventory. Everything else accumulates. The rule it
// implements: a connection may drop bindings only at locations it enumerated in
// full, and claiming a location it merely sampled silently destroys real
// records. These tests pin both halves -- that a claim does drop, and that it
// drops nothing outside itself.

func mergeTestBinding(key string, loc BindingLocation) KeyBindingImpl {
	return KeyBindingImpl{KeyID: ID(key), Location: loc}
}

func mergeTestKeyIDs(bindings []KeyBindingImpl) []string {
	out := make([]string, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, string(b.KeyID)+"@"+string(b.Location))
	}
	sort.Strings(out)
	return out
}

func assertMerged(t *testing.T, got []KeyBindingImpl, want ...string) {
	t.Helper()
	sort.Strings(want)
	if g := mergeTestKeyIDs(got); !reflect.DeepEqual(g, want) {
		t.Errorf("merged bindings = %v, want %v", g, want)
	}
}

// The point of the mechanism: a key taken off a host stops being recorded there.
func TestMergeBindingsDropsAnUnseenBindingAtAClaimedLocation(t *testing.T) {
	existing := []KeyBindingImpl{
		mergeTestBinding("gone", AUTHORIZED_KEYS),
		mergeTestBinding("still-there", AUTHORIZED_KEYS),
	}
	observed := []KeyBindingImpl{mergeTestBinding("still-there", AUTHORIZED_KEYS)}

	got := mergeBindings(existing, observed, []BindingLocation{AUTHORIZED_KEYS})

	assertMerged(t, got, "still-there@AUTHORIZED_KEYS")
}

// The counterweight, and the more dangerous direction: one account legitimately
// carries bindings from several sources. An SSH fetch that enumerated
// authorized_keys knows nothing about AWS instance credentials, and must not
// delete them.
func TestMergeBindingsKeepsBindingsAtUnclaimedLocations(t *testing.T) {
	existing := []KeyBindingImpl{
		mergeTestBinding("ssh-key", AUTHORIZED_KEYS),
		mergeTestBinding("aws-key", AWS_CREDENTIALS),
		mergeTestBinding("instance-key", INSTANCE_ROOT_CREDENTIALS),
	}
	// An SSH fetch: authorized_keys is now empty, and it saw nothing else.
	got := mergeBindings(existing, nil, []BindingLocation{AUTHORIZED_KEYS})

	assertMerged(t, got, "aws-key@CREDENTIALS", "instance-key@INSTANCE ROOT")
}

// An emptied authorized_keys has to be able to clear what was recorded, which
// is why connections report an account even when it has no keys.
func TestMergeBindingsClaimedLocationWithNoObservationsClearsIt(t *testing.T) {
	existing := []KeyBindingImpl{
		mergeTestBinding("a", AUTHORIZED_KEYS),
		mergeTestBinding("b", AUTHORIZED_KEYS),
	}

	if got := mergeBindings(existing, nil, []BindingLocation{AUTHORIZED_KEYS}); len(got) != 0 {
		t.Errorf("merged = %v, want empty -- an emptied authorized_keys must clear its bindings", got)
	}
}

// Claiming nothing is the AWS and Digital Ocean case: fetchKeyPairs emits one
// account per region, each complete for its region and partial for the account,
// so claiming would delete 29 regions' worth of bindings.
func TestMergeBindingsWithNoClaimDeletesNothing(t *testing.T) {
	existing := []KeyBindingImpl{
		mergeTestBinding("from-another-region", AUTHORIZED_KEYS),
	}
	observed := []KeyBindingImpl{mergeTestBinding("from-this-region", AUTHORIZED_KEYS)}

	got := mergeBindings(existing, observed, nil)

	assertMerged(t, got, "from-another-region@AUTHORIZED_KEYS", "from-this-region@AUTHORIZED_KEYS")
}

// A binding seen again at a claimed location survives, exactly once.
func TestMergeBindingsDoesNotDuplicateAReobservedBinding(t *testing.T) {
	b := mergeTestBinding("same", AUTHORIZED_KEYS)

	got := mergeBindings([]KeyBindingImpl{b}, []KeyBindingImpl{b}, []BindingLocation{AUTHORIZED_KEYS})

	assertMerged(t, got, "same@AUTHORIZED_KEYS")
}

// SSH claims AUTHORIZED_KEYS and UnspecifiedLocation together so that records
// written before locksmith stored a location converge on the first fetch after
// an upgrade instead of sitting beside the new ones forever.
func TestMergeBindingsClaimingSeveralLocationsConvergesLegacyRecords(t *testing.T) {
	existing := []KeyBindingImpl{
		mergeTestBinding("legacy", UnspecifiedLocation),
		mergeTestBinding("aws", AWS_CREDENTIALS),
	}
	observed := []KeyBindingImpl{mergeTestBinding("legacy", AUTHORIZED_KEYS)}

	got := mergeBindings(existing, observed, []BindingLocation{AUTHORIZED_KEYS, UnspecifiedLocation})

	assertMerged(t, got, "aws@CREDENTIALS", "legacy@AUTHORIZED_KEYS")
}

// Bindings differing only in their options are different bindings, so at a
// claimed location the observation wins outright. Otherwise tightening a
// restriction on the host would leave the old, looser record beside the new one
// and locksmith would keep reporting privileges that no longer exist.
func TestMergeBindingsConvergesWhenRestrictionsChange(t *testing.T) {
	existing := []KeyBindingImpl{{
		KeyID: "k", Location: AUTHORIZED_KEYS, Options: `no-pty`,
	}}
	observed := []KeyBindingImpl{{
		KeyID: "k", Location: AUTHORIZED_KEYS, Options: `command="/usr/bin/rrsync -ro /srv",restrict`,
	}}

	got := mergeBindings(existing, observed, []BindingLocation{AUTHORIZED_KEYS})

	if len(got) != 1 {
		t.Fatalf("merged = %v, want one binding -- the old options must not survive", got)
	}
	if got[0].Options != `command="/usr/bin/rrsync -ro /srv",restrict` {
		t.Errorf("options = %q, want the newly observed restrictions", got[0].Options)
	}
}

// Merge must take the authority list from the *incoming* observation, not from
// the stored object. The stored one is last run's claim; using it would let a
// fetch that claimed nothing inherit a claim from the record it is updating and
// delete bindings it never looked at.
func TestAccountMergeUsesTheIncomingObservationsAuthority(t *testing.T) {
	stored := NewSSHAccount("alice", "host.example.com", "conn1", []KeyBindingImpl{
		mergeTestBinding("recorded", AUTHORIZED_KEYS),
	})
	stored.MarkObserved(AUTHORIZED_KEYS)

	// A fresh fetch that claims nothing -- it only sampled.
	incoming := NewSSHAccount("alice", "host.example.com", "conn1", []KeyBindingImpl{
		mergeTestBinding("sampled", AUTHORIZED_KEYS),
	})

	stored.Merge(incoming)

	assertMerged(t, stored.Keys,
		"recorded@AUTHORIZED_KEYS", "sampled@AUTHORIZED_KEYS")
}

// ...and when the incoming observation does claim, the merge drops.
func TestAccountMergeHonoursTheIncomingClaim(t *testing.T) {
	stored := NewSSHAccount("alice", "host.example.com", "conn1", []KeyBindingImpl{
		mergeTestBinding("revoked", AUTHORIZED_KEYS),
		mergeTestBinding("aws", AWS_CREDENTIALS),
	})

	incoming := NewSSHAccount("alice", "host.example.com", "conn1", []KeyBindingImpl{
		mergeTestBinding("kept", AUTHORIZED_KEYS),
	})
	incoming.MarkObserved(AUTHORIZED_KEYS)

	stored.Merge(incoming)

	assertMerged(t, stored.Keys, "aws@CREDENTIALS", "kept@AUTHORIZED_KEYS")
}
