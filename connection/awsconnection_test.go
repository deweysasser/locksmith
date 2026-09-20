package connection

import (
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

// ---------------------------------------------------------------------------
// stubs
//
// Each stub embeds the SDK's interface type so that any method the code under
// test calls but the stub does not implement panics with a nil pointer
// dereference rather than reaching the network.  No stub ever builds a real
// client, so no test can talk to AWS or read credentials.
// ---------------------------------------------------------------------------

type awstestSTS struct {
	stsiface.STSAPI
	identity *sts.GetCallerIdentityOutput
	err      error
	calls    int
}

func (s *awstestSTS) GetCallerIdentity(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	s.calls++
	return s.identity, s.err
}

type awstestIAM struct {
	iamiface.IAMAPI

	aliases    *iam.ListAccountAliasesOutput
	aliasesErr error

	// userPages is returned, page by page, from ListUsersPages
	userPages []*iam.ListUsersOutput
	usersErr  error

	// keyPages maps a requested user name ("" meaning "no UserName given") to
	// the pages ListAccessKeysPages should yield for it.
	keyPages map[string][]*iam.ListAccessKeysOutput
	keysErr  error

	mu           sync.Mutex
	requestedFor []string
}

func (i *awstestIAM) ListAccountAliases(*iam.ListAccountAliasesInput) (*iam.ListAccountAliasesOutput, error) {
	return i.aliases, i.aliasesErr
}

func (i *awstestIAM) ListUsersPages(_ *iam.ListUsersInput, fn func(*iam.ListUsersOutput, bool) bool) error {
	if i.usersErr != nil {
		return i.usersErr
	}
	for n, page := range i.userPages {
		if !fn(page, n == len(i.userPages)-1) {
			break
		}
	}
	return nil
}

func (i *awstestIAM) ListAccessKeysPages(in *iam.ListAccessKeysInput, fn func(*iam.ListAccessKeysOutput, bool) bool) error {
	user := aws.StringValue(in.UserName)

	i.mu.Lock()
	i.requestedFor = append(i.requestedFor, user)
	i.mu.Unlock()

	if i.keysErr != nil {
		return i.keysErr
	}
	pages := i.keyPages[user]
	for n, page := range pages {
		if !fn(page, n == len(pages)-1) {
			break
		}
	}
	return nil
}

func (i *awstestIAM) requested() []string {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := append([]string{}, i.requestedFor...)
	sort.Strings(out)
	return out
}

type awstestEC2 struct {
	ec2iface.EC2API

	keyPairs    *ec2.DescribeKeyPairsOutput
	keyPairsErr error

	instancePages []*ec2.DescribeInstancesOutput
	instancesErr  error
}

func (e *awstestEC2) DescribeKeyPairs(*ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error) {
	return e.keyPairs, e.keyPairsErr
}

func (e *awstestEC2) DescribeInstancesPages(_ *ec2.DescribeInstancesInput, fn func(*ec2.DescribeInstancesOutput, bool) bool) error {
	if e.instancesErr != nil {
		return e.instancesErr
	}
	for n, page := range e.instancePages {
		if !fn(page, n == len(e.instancePages)-1) {
			break
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// awstestQuiet silences everything but output.Error (which ignores the level)
// for the duration of a test.
func awstestQuiet(t *testing.T) {
	t.Helper()
	old := output.Level
	output.Level = output.ErrorLevel
	t.Cleanup(func() { output.Level = old })
}

// awstestCollect runs f with a key channel and an account channel, drains both
// concurrently, and returns everything f pushed.  It fails the test if f does
// not return within a few seconds (i.e. if it deadlocks).
func awstestCollect(t *testing.T, f func(keys chan<- data.Key, accounts chan<- data.Account)) ([]data.Key, []data.Account) {
	t.Helper()

	keys := make(chan data.Key)
	accounts := make(chan data.Account)

	var gotKeys []data.Key
	var gotAccounts []data.Account

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for k := range keys {
			gotKeys = append(gotKeys, k)
		}
	}()
	go func() {
		defer wg.Done()
		for a := range accounts {
			gotAccounts = append(gotAccounts, a)
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		f(keys, accounts)
		close(keys)
		close(accounts)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("fetch function did not return")
	}

	wg.Wait()

	return gotKeys, gotAccounts
}

func awstestConn() *AWSConnection {
	return &AWSConnection{Type: "AWSConnection", Profile: "testprofile"}
}

func awstestUser(name, arn string) *iam.User {
	return &iam.User{
		UserName:   aws.String(name),
		Arn:        aws.String(arn),
		CreateDate: aws.Time(time.Unix(1000, 0)),
	}
}

func awstestKeyMD(id, user, status string) *iam.AccessKeyMetadata {
	return &iam.AccessKeyMetadata{
		AccessKeyId: aws.String(id),
		UserName:    aws.String(user),
		Status:      aws.String(status),
		CreateDate:  aws.Time(time.Unix(2000, 0)),
	}
}

func awstestKeyIDs(keys []data.Key) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, string(k.Id()))
	}
	sort.Strings(out)
	return out
}

func awstestAccountIDs(accounts []data.Account) []string {
	out := make([]string, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, string(a.Id()))
	}
	sort.Strings(out)
	return out
}

func awstestBindings(t *testing.T, a data.Account) []data.KeyBindingImpl {
	t.Helper()
	out := make([]data.KeyBindingImpl, 0)
	for b := range a.Bindings() {
		out = append(out, b)
	}
	return out
}

// ---------------------------------------------------------------------------
// fetchAccountInfo
// ---------------------------------------------------------------------------

func Test_fetchAccountInfo(t *testing.T) {
	awstestQuiet(t)

	tests := []struct {
		name        string
		sts         *awstestSTS
		iam         *awstestIAM
		wantArn     data.AWSAccountID
		wantAliases []string
	}{
		{
			name: "aliases are reported",
			sts:  &awstestSTS{identity: &sts.GetCallerIdentityOutput{Account: aws.String("123456789012")}},
			iam: &awstestIAM{aliases: &iam.ListAccountAliasesOutput{
				AccountAliases: aws.StringSlice([]string{"alpha", "beta"}),
			}},
			wantArn:     data.AWSAccountID("123456789012"),
			wantAliases: []string{"alpha", "beta"},
		},
		{
			name:        "no aliases",
			sts:         &awstestSTS{identity: &sts.GetCallerIdentityOutput{Account: aws.String("123456789012")}},
			iam:         &awstestIAM{aliases: &iam.ListAccountAliasesOutput{}},
			wantArn:     data.AWSAccountID("123456789012"),
			wantAliases: []string{},
		},
		{
			name:        "alias lookup fails but account is still reported",
			sts:         &awstestSTS{identity: &sts.GetCallerIdentityOutput{Account: aws.String("123456789012")}},
			iam:         &awstestIAM{aliasesErr: errors.New("access denied")},
			wantArn:     data.AWSAccountID("123456789012"),
			wantAliases: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := awstestConn()

			var arn data.AWSAccountID
			var err error

			keys, accounts := awstestCollect(t, func(_ chan<- data.Key, acc chan<- data.Account) {
				arn, err = a.fetchAccountInfo(tt.sts, tt.iam, acc)
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if arn != tt.wantArn {
				t.Errorf("arn = %q, want %q", arn, tt.wantArn)
			}
			if len(keys) != 0 {
				t.Errorf("expected no keys, got %d", len(keys))
			}
			if len(accounts) != 1 {
				t.Fatalf("expected 1 account, got %d", len(accounts))
			}

			acct, ok := accounts[0].(*data.AWSAccount)
			if !ok {
				t.Fatalf("expected *data.AWSAccount, got %T", accounts[0])
			}
			if acct.Arn != tt.wantArn {
				t.Errorf("account arn = %q, want %q", acct.Arn, tt.wantArn)
			}
			if acct.ConnectionID() != a.Id() {
				t.Errorf("connection id = %q, want %q", acct.ConnectionID(), a.Id())
			}
			if acct.Aliases.Count() != len(tt.wantAliases) {
				t.Errorf("alias count = %d, want %d (%v)", acct.Aliases.Count(), len(tt.wantAliases), acct.Aliases.StringArray())
			}
			for _, want := range tt.wantAliases {
				if !acct.Aliases.Contains(want) {
					t.Errorf("aliases %v missing %q", acct.Aliases.StringArray(), want)
				}
			}
			if acct.Aliases.Contains("") {
				t.Error("alias set contains an empty string")
			}
			if len(acct.Keys) != 0 {
				t.Errorf("expected no key bindings, got %v", acct.Keys)
			}
		})
	}
}

func Test_fetchAccountInfoIdentityFailure(t *testing.T) {
	awstestQuiet(t)

	a := awstestConn()
	s := &awstestSTS{err: errors.New("no credentials")}

	var arn data.AWSAccountID
	var err error

	keys, accounts := awstestCollect(t, func(_ chan<- data.Key, acc chan<- data.Account) {
		arn, err = a.fetchAccountInfo(s, &awstestIAM{}, acc)
	})

	if err == nil {
		t.Fatal("expected an error")
	}
	if arn != "" {
		t.Errorf("arn = %q, want empty", arn)
	}
	if len(keys) != 0 || len(accounts) != 0 {
		t.Errorf("expected nothing emitted, got %d keys and %d accounts", len(keys), len(accounts))
	}
}

// ---------------------------------------------------------------------------
// fetchAccounts
// ---------------------------------------------------------------------------

func Test_fetchAccounts(t *testing.T) {
	awstestQuiet(t)

	tests := []struct {
		name  string
		iam   *awstestIAM
		want  []string
		wants map[string]string // user name -> arn
	}{
		{
			name: "single page",
			iam: &awstestIAM{userPages: []*iam.ListUsersOutput{
				{Users: []*iam.User{awstestUser("alice", "arn:aws:iam::1:user/alice")}},
			}},
			want:  []string{"alice"},
			wants: map[string]string{"alice": "arn:aws:iam::1:user/alice"},
		},
		{
			name: "users past the first page are not lost",
			iam: &awstestIAM{userPages: []*iam.ListUsersOutput{
				{Users: []*iam.User{awstestUser("alice", "arn:aws:iam::1:user/alice")}, IsTruncated: aws.Bool(true), Marker: aws.String("next")},
				{Users: []*iam.User{awstestUser("bob", "arn:aws:iam::1:user/bob")}},
			}},
			want:  []string{"alice", "bob"},
			wants: map[string]string{"bob": "arn:aws:iam::1:user/bob"},
		},
		{
			name: "no users",
			iam:  &awstestIAM{userPages: []*iam.ListUsersOutput{{}}},
			want: []string{},
		},
		{
			name: "list failure degrades to an empty map",
			iam:  &awstestIAM{usersErr: errors.New("access denied")},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := awstestConn()

			var got userMap
			keys, accounts := awstestCollect(t, func(k chan<- data.Key, acc chan<- data.Account) {
				got = a.fetchAccounts(tt.iam, acc, k)
			})

			if len(keys) != 0 || len(accounts) != 0 {
				t.Errorf("fetchAccounts should emit nothing; got %d keys, %d accounts", len(keys), len(accounts))
			}
			if got == nil {
				t.Fatal("expected a non-nil user map")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("user map = %v, want %v", got, tt.want)
			}
			for _, name := range tt.want {
				if _, ok := got[name]; !ok {
					t.Errorf("user map is missing %q", name)
				}
			}
			for name, arn := range tt.wants {
				if aws.StringValue(got[name].Arn) != arn {
					t.Errorf("%s arn = %q, want %q", name, aws.StringValue(got[name].Arn), arn)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// fetchAccessKeys
// ---------------------------------------------------------------------------

func Test_fetchAccessKeysIsPerUser(t *testing.T) {
	awstestQuiet(t)

	a := awstestConn()

	alice := awstestUser("alice", "arn:aws:iam::1:user/alice")
	bob := awstestUser("bob", "arn:aws:iam::1:user/bob")

	i := &awstestIAM{keyPages: map[string][]*iam.ListAccessKeysOutput{
		"alice": {{AccessKeyMetadata: []*iam.AccessKeyMetadata{awstestKeyMD("AKIAALICE", "alice", "Active")}}},
		"bob":   {{AccessKeyMetadata: []*iam.AccessKeyMetadata{awstestKeyMD("AKIABOB", "bob", "Inactive")}}},
	}}

	usermap := userMap{"alice": alice, "bob": bob}

	keys, accounts := awstestCollect(t, func(k chan<- data.Key, acc chan<- data.Account) {
		a.fetchAccessKeys(i, acc, k, usermap)
	})

	// Every user must be asked for explicitly -- an unqualified ListAccessKeys
	// only ever returns the calling user's keys.
	if want := []string{"alice", "bob"}; !awstestEqual(i.requested(), want) {
		t.Errorf("ListAccessKeys called for %v, want %v", i.requested(), want)
	}

	if want := []string{"AKIAALICE", "AKIABOB"}; !awstestEqual(awstestKeyIDs(keys), want) {
		t.Errorf("keys = %v, want %v", awstestKeyIDs(keys), want)
	}
	if want := []string{"arn:aws:iam::1:user/alice", "arn:aws:iam::1:user/bob"}; !awstestEqual(awstestAccountIDs(accounts), want) {
		t.Errorf("accounts = %v, want %v", awstestAccountIDs(accounts), want)
	}

	for _, k := range keys {
		ak, ok := k.(*data.AWSKey)
		if !ok {
			t.Fatalf("expected *data.AWSKey, got %T", k)
		}
		switch ak.AwsKeyId {
		case "AKIAALICE":
			if !ak.Active {
				t.Error("AKIAALICE should be active")
			}
			if !ak.Names.Contains("alice") || !ak.Names.Contains("arn:aws:iam::1:user/alice") {
				t.Errorf("AKIAALICE names = %v", ak.Names.StringArray())
			}
		case "AKIABOB":
			if ak.Active {
				t.Error("AKIABOB should not be active")
			}
		}
		if ak.AwsSecretKey != "" {
			t.Error("secret key material must never be populated")
		}
	}

	for _, acct := range accounts {
		iamAcct, ok := acct.(*data.AWSIamAccount)
		if !ok {
			t.Fatalf("expected *data.AWSIamAccount, got %T", acct)
		}
		bindings := awstestBindings(t, iamAcct)
		if len(bindings) != 1 {
			t.Fatalf("expected 1 binding for %s, got %v", iamAcct.Username, bindings)
		}
		if iamAcct.ConnectionID() != a.Id() {
			t.Errorf("connection id = %q, want %q", iamAcct.ConnectionID(), a.Id())
		}
	}
}

func Test_fetchAccessKeysPaginates(t *testing.T) {
	awstestQuiet(t)

	a := awstestConn()
	alice := awstestUser("alice", "arn:aws:iam::1:user/alice")

	i := &awstestIAM{keyPages: map[string][]*iam.ListAccessKeysOutput{
		"alice": {
			{
				AccessKeyMetadata: []*iam.AccessKeyMetadata{awstestKeyMD("AKIAONE", "alice", "Active")},
				IsTruncated:       aws.Bool(true),
				Marker:            aws.String("next"),
			},
			{AccessKeyMetadata: []*iam.AccessKeyMetadata{awstestKeyMD("AKIATWO", "alice", "Active")}},
		},
	}}

	keys, accounts := awstestCollect(t, func(k chan<- data.Key, acc chan<- data.Account) {
		a.fetchAccessKeys(i, acc, k, userMap{"alice": alice})
	})

	if want := []string{"AKIAONE", "AKIATWO"}; !awstestEqual(awstestKeyIDs(keys), want) {
		t.Errorf("keys = %v, want %v", awstestKeyIDs(keys), want)
	}
	if len(accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(accounts))
	}
}

func Test_fetchAccessKeysUnknownUserDoesNotPanic(t *testing.T) {
	awstestQuiet(t)

	a := awstestConn()

	// An empty user map means ListUsers failed (or returned nothing), so we
	// fall back to the unqualified call -- whose results name a user we know
	// nothing about.
	i := &awstestIAM{keyPages: map[string][]*iam.ListAccessKeysOutput{
		"": {{AccessKeyMetadata: []*iam.AccessKeyMetadata{awstestKeyMD("AKIAROOT", "root", "Active")}}},
	}}

	keys, accounts := awstestCollect(t, func(k chan<- data.Key, acc chan<- data.Account) {
		a.fetchAccessKeys(i, acc, k, userMap{})
	})

	if want := []string{""}; !awstestEqual(i.requested(), want) {
		t.Errorf("ListAccessKeys called for %v, want the unqualified call", i.requested())
	}
	if want := []string{"AKIAROOT"}; !awstestEqual(awstestKeyIDs(keys), want) {
		t.Errorf("keys = %v, want %v", awstestKeyIDs(keys), want)
	}
	if len(accounts) != 0 {
		t.Errorf("expected no IAM accounts for an unknown user, got %v", awstestAccountIDs(accounts))
	}

	ak := keys[0].(*data.AWSKey)
	if !ak.Names.Contains("root") {
		t.Errorf("key names = %v, want to contain the user name", ak.Names.StringArray())
	}
}

func Test_fetchAccessKeysErrorAndEmpty(t *testing.T) {
	awstestQuiet(t)

	tests := []struct {
		name string
		iam  *awstestIAM
	}{
		{"list failure", &awstestIAM{keysErr: errors.New("access denied")}},
		{"no keys", &awstestIAM{keyPages: map[string][]*iam.ListAccessKeysOutput{
			"alice": {{}},
		}}},
		{"no pages at all", &awstestIAM{keyPages: map[string][]*iam.ListAccessKeysOutput{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := awstestConn()
			usermap := userMap{"alice": awstestUser("alice", "arn:aws:iam::1:user/alice")}

			keys, accounts := awstestCollect(t, func(k chan<- data.Key, acc chan<- data.Account) {
				a.fetchAccessKeys(tt.iam, acc, k, usermap)
			})

			if len(keys) != 0 || len(accounts) != 0 {
				t.Errorf("expected nothing emitted, got %d keys and %d accounts", len(keys), len(accounts))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// fetchKeyPairs
// ---------------------------------------------------------------------------

func Test_fetchKeyPairs(t *testing.T) {
	awstestQuiet(t)

	a := awstestConn()
	e := &awstestEC2{keyPairs: &ec2.DescribeKeyPairsOutput{KeyPairs: []*ec2.KeyPairInfo{
		{KeyName: aws.String("laptop"), KeyFingerprint: aws.String("aa:bb:cc")},
		{KeyName: aws.String("build"), KeyFingerprint: aws.String("dd:ee:ff")},
	}}}

	var keymap map[string]data.ID
	keys, accounts := awstestCollect(t, func(k chan<- data.Key, acc chan<- data.Account) {
		keymap = a.fetchKeyPairs(e, data.AWSAccountID("123456789012"), "us-west-2", k, acc)
	})

	want := map[string]data.ID{"laptop": "aa:bb:cc", "build": "dd:ee:ff"}
	if len(keymap) != len(want) {
		t.Fatalf("keymap = %v, want %v", keymap, want)
	}
	for name, id := range want {
		if keymap[name] != id {
			t.Errorf("keymap[%q] = %q, want %q", name, keymap[name], id)
		}
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	for _, k := range keys {
		sk, ok := k.(*data.SSHKey)
		if !ok {
			t.Fatalf("expected *data.SSHKey, got %T", k)
		}
		if sk.PublicKey.Key != nil {
			t.Error("key pairs carry no public key material, only a fingerprint")
		}
		id, ok := want[sk.Names.StringArray()[0]]
		if !ok {
			t.Fatalf("unexpected key name %v", sk.Names.StringArray())
		}
		if !sk.Ids.Contains(id) {
			t.Errorf("key %v does not carry fingerprint %q", sk.Names.StringArray(), id)
		}
		if sk.Id() != id {
			t.Errorf("key id = %q, want the fingerprint %q", sk.Id(), id)
		}
	}

	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	acct := accounts[0].(*data.AWSAccount)
	if acct.Arn != data.AWSAccountID("123456789012") {
		t.Errorf("account arn = %q", acct.Arn)
	}
	bindings := awstestBindings(t, acct)
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %v", bindings)
	}
	for _, b := range bindings {
		if want[b.Name] != b.KeyID {
			t.Errorf("binding %v does not match the keymap", b)
		}
		if b.Location != "" {
			t.Errorf("key pair binding location = %q, want unset", b.Location)
		}
	}
}

func Test_fetchKeyPairsEmpty(t *testing.T) {
	awstestQuiet(t)

	a := awstestConn()
	e := &awstestEC2{keyPairs: &ec2.DescribeKeyPairsOutput{}}

	var keymap map[string]data.ID
	keys, accounts := awstestCollect(t, func(k chan<- data.Key, acc chan<- data.Account) {
		keymap = a.fetchKeyPairs(e, data.AWSAccountID("1"), "eu-west-1", k, acc)
	})

	if len(keymap) != 0 {
		t.Errorf("keymap = %v, want empty", keymap)
	}
	if len(keys) != 0 {
		t.Errorf("expected no keys, got %d", len(keys))
	}
	if len(accounts) != 1 {
		t.Fatalf("expected the (empty) account to still be reported, got %d", len(accounts))
	}
	if bindings := awstestBindings(t, accounts[0]); len(bindings) != 0 {
		t.Errorf("expected no bindings, got %v", bindings)
	}
}

func Test_fetchKeyPairsError(t *testing.T) {
	awstestQuiet(t)

	a := awstestConn()
	e := &awstestEC2{keyPairsErr: errors.New("region not enabled")}

	var keymap map[string]data.ID
	keys, accounts := awstestCollect(t, func(k chan<- data.Key, acc chan<- data.Account) {
		keymap = a.fetchKeyPairs(e, data.AWSAccountID("1"), "ap-south-1", k, acc)
	})

	if keymap == nil {
		t.Error("expected a non-nil keymap even on failure")
	}
	if len(keymap) != 0 {
		t.Errorf("keymap = %v, want empty", keymap)
	}
	if len(keys) != 0 || len(accounts) != 0 {
		t.Errorf("expected nothing emitted, got %d keys and %d accounts", len(keys), len(accounts))
	}
}

// ---------------------------------------------------------------------------
// fetchInstances
// ---------------------------------------------------------------------------

func awstestInstance(id, keyName, dns, nameTag string) *ec2.Instance {
	i := &ec2.Instance{
		InstanceId:    aws.String(id),
		PublicDnsName: aws.String(dns),
	}
	if keyName != "" {
		i.KeyName = aws.String(keyName)
	}
	if nameTag != "" {
		i.Tags = []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String(nameTag)}}
	}
	return i
}

func Test_fetchInstances(t *testing.T) {
	awstestQuiet(t)

	keymap := map[string]data.ID{"laptop": "aa:bb:cc"}

	tests := []struct {
		name         string
		ec2          *awstestEC2
		wantInstance []string
		wantKeyIDs   map[string]data.ID
	}{
		{
			name: "instance key name maps through the keymap",
			ec2: &awstestEC2{instancePages: []*ec2.DescribeInstancesOutput{
				{Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{
					awstestInstance("i-1", "laptop", "one.example.com", "one"),
				}}}},
			}},
			wantInstance: []string{"i-1"},
			wantKeyIDs:   map[string]data.ID{"i-1": "aa:bb:cc"},
		},
		{
			name: "an unknown key name yields an empty binding id",
			ec2: &awstestEC2{instancePages: []*ec2.DescribeInstancesOutput{
				{Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{
					awstestInstance("i-2", "unknown", "two.example.com", ""),
				}}}},
			}},
			wantInstance: []string{"i-2"},
			wantKeyIDs:   map[string]data.ID{"i-2": ""},
		},
		{
			name: "instances past the first page are not lost",
			ec2: &awstestEC2{instancePages: []*ec2.DescribeInstancesOutput{
				{
					Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{
						awstestInstance("i-1", "laptop", "one.example.com", "one"),
					}}},
					NextToken: aws.String("next"),
				},
				{Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{
					awstestInstance("i-3", "laptop", "three.example.com", ""),
				}}}},
			}},
			wantInstance: []string{"i-1", "i-3"},
			wantKeyIDs:   map[string]data.ID{"i-1": "aa:bb:cc", "i-3": "aa:bb:cc"},
		},
		{
			name:         "no instances",
			ec2:          &awstestEC2{instancePages: []*ec2.DescribeInstancesOutput{{}}},
			wantInstance: []string{},
		},
		{
			name:         "describe failure",
			ec2:          &awstestEC2{instancesErr: errors.New("throttled")},
			wantInstance: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := awstestConn()

			_, accounts := awstestCollect(t, func(_ chan<- data.Key, acc chan<- data.Account) {
				a.fetchInstances(tt.ec2, "us-east-1", acc, keymap)
			})

			if !awstestEqual(awstestAccountIDs(accounts), tt.wantInstance) {
				t.Fatalf("accounts = %v, want %v", awstestAccountIDs(accounts), tt.wantInstance)
			}

			for _, acct := range accounts {
				inst, ok := acct.(*data.AWSInstanceAccount)
				if !ok {
					t.Fatalf("expected *data.AWSInstanceAccount, got %T", acct)
				}
				bindings := awstestBindings(t, inst)
				if len(bindings) != 1 {
					t.Fatalf("expected exactly 1 binding for %s, got %v", inst.InstanceId, bindings)
				}
				if bindings[0].Location != data.INSTANCE_ROOT_CREDENTIALS {
					t.Errorf("binding location = %q, want %q", bindings[0].Location, data.INSTANCE_ROOT_CREDENTIALS)
				}
				if want := tt.wantKeyIDs[inst.InstanceId]; bindings[0].KeyID != want {
					t.Errorf("%s binding key id = %q, want %q", inst.InstanceId, bindings[0].KeyID, want)
				}
				if inst.ConnectionID() != a.Id() {
					t.Errorf("connection id = %q, want %q", inst.ConnectionID(), a.Id())
				}
			}
		})
	}
}

func Test_fetchInstancesNoKeyName(t *testing.T) {
	awstestQuiet(t)

	a := awstestConn()
	e := &awstestEC2{instancePages: []*ec2.DescribeInstancesOutput{
		{Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{
			awstestInstance("i-nokey", "", "nokey.example.com", ""),
		}}}},
	}}

	_, accounts := awstestCollect(t, func(_ chan<- data.Key, acc chan<- data.Account) {
		a.fetchInstances(e, "us-east-1", acc, map[string]data.ID{"laptop": "aa:bb:cc"})
	})

	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	bindings := awstestBindings(t, accounts[0])
	if len(bindings) != 1 || bindings[0].KeyID != "" {
		t.Errorf("bindings = %v, want a single empty binding", bindings)
	}
}

// awstestEqual compares two string slices for equality.
func awstestEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
