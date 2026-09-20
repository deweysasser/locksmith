package data

import (
	"reflect"
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

	a.AddBinding(key)

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

	first := mergeBindings(b1, b2)
	for i := 0; i < 20; i++ {
		if got := mergeBindings(b1, b2); !reflect.DeepEqual(got, first) {
			t.Fatalf("merge order is unstable:\n run 1: %v\n run %d: %v", first, i+2, got)
		}
	}

	if len(first) != 3 {
		t.Fatalf("got %d bindings, want 3", len(first))
	}
}

func TestMergeBindingsEmptyInputs(t *testing.T) {
	if got := mergeBindings(nil, nil); len(got) != 0 {
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
