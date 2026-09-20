package connection

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"github.com/digitalocean/godo"
	"golang.org/x/crypto/ssh"
)

// ---------------------------------------------------------------------------
// stubs
//
// The droplet and key stubs embed godo's service interfaces so that any method
// the code under test calls but the stub does not implement panics rather than
// reaching the network.  No stub builds a real client, so no test here can talk
// to Digital Ocean or read a token.
// ---------------------------------------------------------------------------

type dotestAccountSvc struct {
	account *godo.Account
	err     error
	calls   int
}

func (s *dotestAccountSvc) Get(context.Context) (*godo.Account, *godo.Response, error) {
	s.calls++
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.account, &godo.Response{}, nil
}

type dotestKeysSvc struct {
	godo.KeysService

	pages [][]godo.Key
	// alwaysMore makes every response claim there is another page, which is
	// what a malformed API would look like.
	alwaysMore bool
	err        error

	mu        sync.Mutex
	requested []int
}

func (s *dotestKeysSvc) List(_ context.Context, opt *godo.ListOptions) ([]godo.Key, *godo.Response, error) {
	page := dotestRecord(&s.mu, &s.requested, opt)

	if s.err != nil {
		return nil, nil, s.err
	}

	if s.alwaysMore {
		return s.pages[0], dotestResponse(1, 2), nil
	}

	if page > len(s.pages) {
		return nil, dotestResponse(page, page), nil
	}

	return s.pages[page-1], dotestResponse(page, len(s.pages)), nil
}

type dotestDropletsSvc struct {
	godo.DropletsService

	pages [][]godo.Droplet
	err   error

	mu        sync.Mutex
	requested []int
}

func (s *dotestDropletsSvc) List(_ context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	page := dotestRecord(&s.mu, &s.requested, opt)

	if s.err != nil {
		return nil, nil, s.err
	}

	if page > len(s.pages) {
		return nil, dotestResponse(page, page), nil
	}

	return s.pages[page-1], dotestResponse(page, len(s.pages)), nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// dotestRecord notes which page was asked for and returns it, 1 based.
func dotestRecord(mu *sync.Mutex, requested *[]int, opt *godo.ListOptions) int {
	page := 1
	if opt != nil && opt.Page > 0 {
		page = opt.Page
	}

	mu.Lock()
	*requested = append(*requested, page)
	mu.Unlock()

	return page
}

// dotestResponse builds the pagination links Digital Ocean returns in the body
// for page "page" of "total" pages.
func dotestResponse(page, total int) *godo.Response {
	if total <= 1 {
		return &godo.Response{}
	}

	pages := &godo.Pages{}
	if page > 1 {
		pages.Prev = fmt.Sprintf("https://api.example.invalid/v2/things?page=%d", page-1)
	}
	if page < total {
		pages.Next = fmt.Sprintf("https://api.example.invalid/v2/things?page=%d", page+1)
	}

	return &godo.Response{Links: &godo.Links{Pages: pages}}
}

// dotestQuiet silences everything but output.Error (which ignores the level)
// for the duration of a test.
func dotestQuiet(t *testing.T) {
	t.Helper()
	old := output.Level
	output.Level = output.ErrorLevel
	t.Cleanup(func() { output.Level = old })
}

// dotestCaptureOutput collects everything the output package prints while fn
// runs.  output.ErrorCount() cannot be used for this: it closes the counting
// channel, which would break every later output call in the process.
func dotestCaptureOutput(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal("could not make a pipe:", err)
	}
	os.Stdout = w

	captured := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		captured <- string(b)
	}()

	fn()

	os.Stdout = old
	if err := w.Close(); err != nil {
		t.Fatal("could not close the pipe:", err)
	}
	s := <-captured
	if err := r.Close(); err != nil {
		t.Fatal("could not close the pipe:", err)
	}

	return s
}

// dotestCollect runs f with a key channel and an account channel, drains both
// concurrently, and returns everything f pushed.  It fails the test if f does
// not return within a few seconds (i.e. if it deadlocks or spins).
func dotestCollect(t *testing.T, f func(keys chan<- data.Key, accounts chan<- data.Account)) ([]data.Key, []data.Account) {
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

func dotestConn() *DOConnection {
	return &DOConnection{Type: "DOConnection", Name: "testaccount"}
}

// dotestPublicKey returns a deterministic, entirely synthetic ed25519 public
// key, so that no test needs real key material on disk.
func dotestPublicKey(t *testing.T, seed byte) ssh.PublicKey {
	t.Helper()

	raw := make([]byte, ed25519.SeedSize)
	for i := range raw {
		raw[i] = seed
	}

	pub, err := ssh.NewPublicKey(ed25519.NewKeyFromSeed(raw).Public())
	if err != nil {
		t.Fatal("could not build a test public key:", err)
	}

	return pub
}

// dotestGodoKey builds the Digital Ocean representation of a public key: the
// authorized_keys line plus the MD5 fingerprint the API reports.
func dotestGodoKey(id int, name string, pub ssh.PublicKey) godo.Key {
	return godo.Key{
		ID:          id,
		Name:        name,
		Fingerprint: ssh.FingerprintLegacyMD5(pub),
		PublicKey:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))),
	}
}

func dotestDroplet(id int, name, publicIP, privateIP string) godo.Droplet {
	networks := &godo.Networks{}
	if publicIP != "" {
		networks.V4 = append(networks.V4, godo.NetworkV4{Type: "public", IPAddress: publicIP})
	}
	if privateIP != "" {
		networks.V4 = append(networks.V4, godo.NetworkV4{Type: "private", IPAddress: privateIP})
	}

	return godo.Droplet{ID: id, Name: name, Networks: networks}
}

func dotestKeyIDs(keys []data.Key) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, string(k.Id()))
	}
	sort.Strings(out)
	return out
}

func dotestAccountIDs(accounts []data.Account) []string {
	out := make([]string, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, string(a.Id()))
	}
	sort.Strings(out)
	return out
}

func dotestBindings(a data.Account) []data.KeyBindingImpl {
	out := make([]data.KeyBindingImpl, 0)
	for b := range a.Bindings() {
		out = append(out, b)
	}
	return out
}

func dotestEqual(a, b []string) bool {
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

func dotestOnlyDOAccount(t *testing.T, accounts []data.Account) *DOAccount {
	t.Helper()

	var found *DOAccount
	for _, a := range accounts {
		if acct, ok := a.(*DOAccount); ok {
			if found != nil {
				t.Fatal("more than one DOAccount was emitted")
			}
			found = acct
		}
	}

	if found == nil {
		t.Fatal("no DOAccount was emitted")
	}

	return found
}

// ---------------------------------------------------------------------------
// account info
// ---------------------------------------------------------------------------

func Test_DOFetchAccountInfo(t *testing.T) {
	dotestQuiet(t)

	tests := []struct {
		name          string
		svc           *dotestAccountSvc
		wantID        string
		wantEmail     string
		wantTeam      string
		wantTypeField string
	}{
		{
			name: "full account",
			svc: &dotestAccountSvc{account: &godo.Account{
				UUID:  "11111111111111111111111111111111",
				Email: "nobody@example.invalid",
				Team:  &godo.TeamInfo{Name: "Test Team"},
			}},
			wantID:        "11111111111111111111111111111111",
			wantEmail:     "nobody@example.invalid",
			wantTeam:      "Test Team",
			wantTypeField: "DOAccount",
		},
		{
			name:          "no team",
			svc:           &dotestAccountSvc{account: &godo.Account{UUID: "22222222", Email: "someone@example.invalid"}},
			wantID:        "22222222",
			wantEmail:     "someone@example.invalid",
			wantTypeField: "DOAccount",
		},
		{
			name:          "api error falls back to the connection",
			svc:           &dotestAccountSvc{err: errors.New("401 unauthorized")},
			wantID:        "do:testaccount",
			wantTypeField: "DOAccount",
		},
		{
			name:          "empty response falls back to the connection",
			svc:           &dotestAccountSvc{},
			wantID:        "do:testaccount",
			wantTypeField: "DOAccount",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := dotestConn()
			acct := d.fetchAccountInfo(context.Background(), test.svc)

			if string(acct.Id()) != test.wantID {
				t.Errorf("account id = %s, want %s", acct.Id(), test.wantID)
			}
			if acct.Email != test.wantEmail {
				t.Errorf("email = %s, want %s", acct.Email, test.wantEmail)
			}
			if acct.TeamName != test.wantTeam {
				t.Errorf("team = %s, want %s", acct.TeamName, test.wantTeam)
			}
			if acct.Type != test.wantTypeField {
				t.Errorf("Type field = %s, want %s", acct.Type, test.wantTypeField)
			}
			if acct.ConnectionID() != d.Id() {
				t.Errorf("connection id = %s, want %s", acct.ConnectionID(), d.Id())
			}
			if test.svc.calls != 1 {
				t.Errorf("account service called %d times, want 1", test.svc.calls)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// keys
// ---------------------------------------------------------------------------

func Test_DOFetchKeys(t *testing.T) {
	dotestQuiet(t)

	first := dotestPublicKey(t, 1)
	second := dotestPublicKey(t, 2)

	svc := &dotestKeysSvc{pages: [][]godo.Key{{
		dotestGodoKey(101, "first-key", first),
		dotestGodoKey(102, "second-key", second),
	}}}

	d := dotestConn()
	acct := NewDOAccount(d.Id(), "account-uuid", nil)

	var keymap map[int]data.ID
	keys, accounts := dotestCollect(t, func(cKeys chan<- data.Key, cAccounts chan<- data.Account) {
		keymap = d.fetchKeys(context.Background(), svc, acct, cKeys, cAccounts)
	})

	want := []string{ssh.FingerprintSHA256(first), ssh.FingerprintSHA256(second)}
	sort.Strings(want)
	if got := dotestKeyIDs(keys); !dotestEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}

	// Digital Ocean's fingerprint is the legacy MD5 form, which is one of the
	// identifiers locksmith computes itself -- that is what lets a key found
	// here merge with the same key found in an authorized_keys file.
	for _, k := range keys {
		ids := k.(*data.SSHKey).Identifiers()
		found := false
		for _, id := range ids {
			if string(id) == ssh.FingerprintLegacyMD5(first) || string(id) == ssh.FingerprintLegacyMD5(second) {
				found = true
			}
		}
		if !found {
			t.Errorf("key %s has no MD5 identifier among %v", k.Id(), ids)
		}
	}

	if len(keymap) != 2 {
		t.Fatalf("keymap = %v, want 2 entries", keymap)
	}
	if keymap[101] != data.ID(ssh.FingerprintSHA256(first)) {
		t.Errorf("keymap[101] = %s, want %s", keymap[101], ssh.FingerprintSHA256(first))
	}
	if keymap[102] != data.ID(ssh.FingerprintSHA256(second)) {
		t.Errorf("keymap[102] = %s, want %s", keymap[102], ssh.FingerprintSHA256(second))
	}

	if got := dotestAccountIDs(accounts); !dotestEqual(got, []string{"account-uuid"}) {
		t.Fatalf("accounts = %v, want [account-uuid]", got)
	}

	bindings := dotestBindings(accounts[0])
	if len(bindings) != 2 {
		t.Fatalf("bindings = %v, want 2", bindings)
	}
	names := make([]string, 0, len(bindings))
	for _, b := range bindings {
		names = append(names, b.Name)
		if b.KeyID == "" {
			t.Error("binding has no key id:", b)
		}
	}
	sort.Strings(names)
	if !dotestEqual(names, []string{"first-key", "second-key"}) {
		t.Errorf("binding names = %v, want [first-key second-key]", names)
	}
}

func Test_DOFetchKeysUnparseablePublicKeyFallsBackToFingerprint(t *testing.T) {
	dotestQuiet(t)

	svc := &dotestKeysSvc{pages: [][]godo.Key{{
		// An ECDSA key: valid to Digital Ocean, but not something
		// data.NewKey recognises.
		{ID: 201, Name: "ecdsa-key", Fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
			PublicKey: "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAI"},
		// Neither usable public key material nor a fingerprint: nothing we
		// could record, so it must be skipped rather than stored as a key
		// with no identity.
		{ID: 202, Name: "useless-key"},
	}}}

	d := dotestConn()
	acct := NewDOAccount(d.Id(), "account-uuid", nil)

	var keymap map[int]data.ID
	keys, _ := dotestCollect(t, func(cKeys chan<- data.Key, cAccounts chan<- data.Account) {
		keymap = d.fetchKeys(context.Background(), svc, acct, cKeys, cAccounts)
	})

	if got := dotestKeyIDs(keys); !dotestEqual(got, []string{"aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99"}) {
		t.Fatalf("keys = %v, want just the fingerprint key", got)
	}

	sshKey, ok := keys[0].(*data.SSHKey)
	if !ok {
		t.Fatalf("key was a %T, want *data.SSHKey", keys[0])
	}
	names := sshKey.GetNames()
	if !names.Contains("ecdsa-key") {
		t.Errorf("key names = %v, want to contain ecdsa-key", names.StringArray())
	}

	if len(keymap) != 1 {
		t.Errorf("keymap = %v, want only the key we could identify", keymap)
	}
	if _, ok := keymap[202]; ok {
		t.Error("the key with no identity at all was recorded in the keymap")
	}
}

func Test_DOFetchKeysPaginates(t *testing.T) {
	dotestQuiet(t)

	first := dotestPublicKey(t, 3)
	second := dotestPublicKey(t, 4)
	third := dotestPublicKey(t, 5)

	svc := &dotestKeysSvc{pages: [][]godo.Key{
		{dotestGodoKey(1, "one", first)},
		{dotestGodoKey(2, "two", second)},
		{dotestGodoKey(3, "three", third)},
	}}

	d := dotestConn()
	acct := NewDOAccount(d.Id(), "account-uuid", nil)

	keys, accounts := dotestCollect(t, func(cKeys chan<- data.Key, cAccounts chan<- data.Account) {
		d.fetchKeys(context.Background(), svc, acct, cKeys, cAccounts)
	})

	if len(keys) != 3 {
		t.Errorf("got %d keys, want all 3 pages worth", len(keys))
	}
	if got, want := svc.requested, []int{1, 2, 3}; !dotestEqual(dotestInts(got), dotestInts(want)) {
		t.Errorf("requested pages %v, want %v", got, want)
	}
	if bindings := dotestBindings(accounts[0]); len(bindings) != 3 {
		t.Errorf("account has %d bindings, want 3", len(bindings))
	}
}

func Test_DOFetchKeysMalformedPaginationTerminates(t *testing.T) {
	dotestQuiet(t)

	pub := dotestPublicKey(t, 6)
	svc := &dotestKeysSvc{
		pages:      [][]godo.Key{{dotestGodoKey(1, "one", pub)}},
		alwaysMore: true,
	}

	d := dotestConn()
	acct := NewDOAccount(d.Id(), "account-uuid", nil)

	// A response that always claims to be page 1 of more must not spin: the
	// collector fails the test if fetchKeys does not return.
	dotestCollect(t, func(cKeys chan<- data.Key, cAccounts chan<- data.Account) {
		d.fetchKeys(context.Background(), svc, acct, cKeys, cAccounts)
	})

	if len(svc.requested) > 2 {
		t.Errorf("made %d list calls, want to stop as soon as the page number stops advancing", len(svc.requested))
	}
}

func Test_DOFetchKeysEmpty(t *testing.T) {
	dotestQuiet(t)

	svc := &dotestKeysSvc{pages: [][]godo.Key{{}}}

	d := dotestConn()
	acct := NewDOAccount(d.Id(), "account-uuid", nil)

	keys, accounts := dotestCollect(t, func(cKeys chan<- data.Key, cAccounts chan<- data.Account) {
		d.fetchKeys(context.Background(), svc, acct, cKeys, cAccounts)
	})

	if len(keys) != 0 {
		t.Errorf("got %d keys, want none", len(keys))
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want the account itself", len(accounts))
	}
	if bindings := dotestBindings(accounts[0]); len(bindings) != 0 {
		t.Errorf("bindings = %v, want none", bindings)
	}
}

func Test_DOFetchKeysError(t *testing.T) {
	dotestQuiet(t)

	svc := &dotestKeysSvc{err: errors.New("500 server error")}

	d := dotestConn()
	acct := NewDOAccount(d.Id(), "account-uuid", nil)

	var keymap map[int]data.ID
	var keys []data.Key
	var accounts []data.Account

	printed := dotestCaptureOutput(t, func() {
		keys, accounts = dotestCollect(t, func(cKeys chan<- data.Key, cAccounts chan<- data.Account) {
			keymap = d.fetchKeys(context.Background(), svc, acct, cKeys, cAccounts)
		})
	})

	if len(keys) != 0 {
		t.Errorf("got %d keys, want none", len(keys))
	}
	if len(keymap) != 0 {
		t.Errorf("keymap = %v, want empty", keymap)
	}
	if len(accounts) != 1 {
		t.Errorf("got %d accounts, want the account to be reported anyway", len(accounts))
	}
	if !strings.Contains(printed, "failed to list SSH keys") {
		t.Errorf("a failed key listing reported %q, want it to say what failed", printed)
	}
}

// ---------------------------------------------------------------------------
// droplets
// ---------------------------------------------------------------------------

func Test_DOFetchDroplets(t *testing.T) {
	dotestQuiet(t)

	svc := &dotestDropletsSvc{pages: [][]godo.Droplet{{
		dotestDroplet(900001, "mail.example.invalid", "198.51.100.10", ""),
		dotestDroplet(900002, "worker-pool-abcdef", "198.51.100.11", "10.0.0.11"),
		// A droplet with no networks at all must not take the fetch down.
		{ID: 900003, Name: "no-networks"},
	}}}

	d := dotestConn()

	_, accounts := dotestCollect(t, func(_ chan<- data.Key, cAccounts chan<- data.Account) {
		d.fetchDroplets(context.Background(), svc, map[int]data.ID{}, cAccounts)
	})

	want := []string{"do-droplet-900001", "do-droplet-900002", "do-droplet-900003"}
	if got := dotestAccountIDs(accounts); !dotestEqual(got, want) {
		t.Fatalf("accounts = %v, want %v", got, want)
	}

	byID := make(map[data.ID]*DODropletAccount)
	for _, a := range accounts {
		acct, ok := a.(*DODropletAccount)
		if !ok {
			t.Fatalf("account was a %T, want *DODropletAccount", a)
		}
		if acct.Type != "DODropletAccount" {
			t.Errorf("Type field = %s, want DODropletAccount", acct.Type)
		}
		if acct.ConnectionID() != d.Id() {
			t.Errorf("connection id = %s, want %s", acct.ConnectionID(), d.Id())
		}
		byID[acct.Id()] = acct
	}

	if ip := byID["do-droplet-900002"].PublicIPv4; ip != "198.51.100.11" {
		t.Errorf("public address = %s, want the public one and not the private one", ip)
	}
	if ip := byID["do-droplet-900003"].PublicIPv4; ip != "" {
		t.Errorf("address of a droplet with no networks = %s, want empty", ip)
	}
	if name := byID["do-droplet-900001"].Name; name != "mail.example.invalid" {
		t.Errorf("name = %s, want mail.example.invalid", name)
	}
	if s := byID["do-droplet-900001"].String(); !strings.Contains(s, "mail.example.invalid") || !strings.Contains(s, "198.51.100.10") {
		t.Errorf("String() = %s, want it to mention the name and address", s)
	}
}

func Test_DOFetchDropletsPaginates(t *testing.T) {
	dotestQuiet(t)

	svc := &dotestDropletsSvc{pages: [][]godo.Droplet{
		{dotestDroplet(1, "one", "198.51.100.1", "")},
		{dotestDroplet(2, "two", "198.51.100.2", "")},
	}}

	d := dotestConn()

	_, accounts := dotestCollect(t, func(_ chan<- data.Key, cAccounts chan<- data.Account) {
		d.fetchDroplets(context.Background(), svc, map[int]data.ID{}, cAccounts)
	})

	if len(accounts) != 2 {
		t.Errorf("got %d droplet accounts, want both pages", len(accounts))
	}
	if got, want := svc.requested, []int{1, 2}; !dotestEqual(dotestInts(got), dotestInts(want)) {
		t.Errorf("requested pages %v, want %v", got, want)
	}
}

func Test_DOFetchDropletsEmptyAndError(t *testing.T) {
	dotestQuiet(t)

	t.Run("empty", func(t *testing.T) {
		svc := &dotestDropletsSvc{pages: [][]godo.Droplet{{}}}
		_, accounts := dotestCollect(t, func(_ chan<- data.Key, cAccounts chan<- data.Account) {
			dotestConn().fetchDroplets(context.Background(), svc, map[int]data.ID{}, cAccounts)
		})
		if len(accounts) != 0 {
			t.Errorf("got %d accounts, want none", len(accounts))
		}
	})

	t.Run("error", func(t *testing.T) {
		svc := &dotestDropletsSvc{err: errors.New("503 unavailable")}

		var accounts []data.Account
		printed := dotestCaptureOutput(t, func() {
			_, accounts = dotestCollect(t, func(_ chan<- data.Key, cAccounts chan<- data.Account) {
				dotestConn().fetchDroplets(context.Background(), svc, map[int]data.ID{}, cAccounts)
			})
		})

		if len(accounts) != 0 {
			t.Errorf("got %d accounts, want none", len(accounts))
		}
		if !strings.Contains(printed, "failed to list droplets") {
			t.Errorf("a failed droplet listing reported %q, want it to say what failed", printed)
		}
	})
}

// ---------------------------------------------------------------------------
// droplet key bindings
// ---------------------------------------------------------------------------

func Test_DODropletBindings(t *testing.T) {
	dotestQuiet(t)

	keymap := map[int]data.ID{
		11: data.ID("SHA256:aaaa"),
		12: data.ID("SHA256:bbbb"),
	}

	d := dotestConn()

	t.Run("known keys", func(t *testing.T) {
		bindings := d.dropletBindings("a-droplet", []int{11, 12}, keymap)
		if len(bindings) != 2 {
			t.Fatalf("bindings = %v, want 2", bindings)
		}
		for _, b := range bindings {
			if b.Location != data.INSTANCE_ROOT_CREDENTIALS {
				t.Errorf("location = %s, want %s", b.Location, data.INSTANCE_ROOT_CREDENTIALS)
			}
		}
		if bindings[0].KeyID != "SHA256:aaaa" || bindings[1].KeyID != "SHA256:bbbb" {
			t.Errorf("bindings = %v, want the mapped key ids", bindings)
		}
	})

	t.Run("unknown key is skipped", func(t *testing.T) {
		bindings := d.dropletBindings("a-droplet", []int{11, 99}, keymap)
		if len(bindings) != 1 {
			t.Fatalf("bindings = %v, want only the key we know about", bindings)
		}
		if bindings[0].KeyID != "SHA256:aaaa" {
			t.Errorf("binding = %v, want the known key", bindings[0])
		}
	})

	t.Run("no keys", func(t *testing.T) {
		if bindings := d.dropletBindings("a-droplet", nil, keymap); len(bindings) != 0 {
			t.Errorf("bindings = %v, want none", bindings)
		}
	})
}

// Test_DODropletKeyIDs records what the Digital Ocean API actually tells us
// about which keys are on a droplet: nothing.  ssh_keys is an argument to
// droplet creation and is never echoed back by either droplet endpoint.  If
// this test ever starts failing because godo grew the field, the binding code
// above is ready for it.
func Test_DODropletKeyIDs(t *testing.T) {
	droplet := dotestDroplet(900004, "some-droplet", "198.51.100.4", "")
	if ids := dropletKeyIDs(droplet); len(ids) != 0 {
		t.Errorf("dropletKeyIDs = %v, want none -- the API does not report them", ids)
	}
}

// ---------------------------------------------------------------------------
// the whole fetch, and the client
// ---------------------------------------------------------------------------

func Test_DOFetchEverything(t *testing.T) {
	dotestQuiet(t)

	pub := dotestPublicKey(t, 7)

	accountSvc := &dotestAccountSvc{account: &godo.Account{
		UUID:  "33333333333333333333333333333333",
		Email: "nobody@example.invalid",
	}}
	keySvc := &dotestKeysSvc{pages: [][]godo.Key{{dotestGodoKey(55, "a-key", pub)}}}
	dropletSvc := &dotestDropletsSvc{pages: [][]godo.Droplet{{dotestDroplet(900005, "a-droplet", "198.51.100.5", "")}}}

	d := dotestConn()

	keys, accounts := dotestCollect(t, func(cKeys chan<- data.Key, cAccounts chan<- data.Account) {
		d.fetch(context.Background(), accountSvc, keySvc, dropletSvc, cKeys, cAccounts)
	})

	if got := dotestKeyIDs(keys); !dotestEqual(got, []string{ssh.FingerprintSHA256(pub)}) {
		t.Errorf("keys = %v, want the account's one key", got)
	}

	want := []string{"33333333333333333333333333333333", "do-droplet-900005"}
	if got := dotestAccountIDs(accounts); !dotestEqual(got, want) {
		t.Errorf("accounts = %v, want %v", got, want)
	}

	acct := dotestOnlyDOAccount(t, accounts)
	if bindings := dotestBindings(acct); len(bindings) != 1 {
		t.Errorf("account bindings = %v, want the one key", bindings)
	}
}

func Test_DOFetchWithNoToken(t *testing.T) {
	dotestQuiet(t)

	t.Setenv(DOTokenVariable, "")

	var keys []data.Key
	var accounts []data.Account

	printed := dotestCaptureOutput(t, func() {
		keys, accounts = fetchAll(dotestConn())
	})

	if len(keys) != 0 || len(accounts) != 0 {
		t.Errorf("got %d keys and %d accounts with no token, want none", len(keys), len(accounts))
	}
	if !strings.Contains(printed, DOTokenVariable) {
		t.Errorf("fetching with no token reported %q, want it to name the variable to set", printed)
	}
}

func Test_DOClient(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		t.Setenv(DOTokenVariable, "")
		client, err := doClient()
		if err == nil {
			t.Fatal("a missing token was not an error")
		}
		if client != nil {
			t.Error("a client was built without a token")
		}
		if !strings.Contains(err.Error(), DOTokenVariable) {
			t.Errorf("error %q does not say which variable to set", err)
		}
	})

	t.Run("whitespace only token", func(t *testing.T) {
		t.Setenv(DOTokenVariable, "   ")
		if _, err := doClient(); err == nil {
			t.Fatal("a blank token was not an error")
		}
	})

	t.Run("token present", func(t *testing.T) {
		// Building a client makes no request, so this stays hermetic.
		t.Setenv(DOTokenVariable, "not-a-real-token")
		client, err := doClient()
		if err != nil {
			t.Fatal("building a client failed:", err)
		}
		if client == nil {
			t.Fatal("no client and no error")
		}
	})
}

// ---------------------------------------------------------------------------
// accounts as stored objects
// ---------------------------------------------------------------------------

func Test_DOAccountMerge(t *testing.T) {
	dotestQuiet(t)

	a := NewDOAccount("do:testaccount", "account-uuid", []data.KeyBindingImpl{
		{KeyID: "SHA256:bbbb", Name: "second"},
		{KeyID: "SHA256:aaaa", Name: "first"},
	})
	b := NewDOAccount("do:testaccount", "account-uuid", []data.KeyBindingImpl{
		{KeyID: "SHA256:aaaa", Name: "first"},
		{KeyID: "SHA256:cccc", Name: "third"},
	})
	b.Email = "nobody@example.invalid"
	b.TeamName = "Test Team"

	a.Merge(b)

	ids := make([]string, 0, len(a.Keys))
	for _, k := range a.Keys {
		ids = append(ids, string(k.KeyID))
	}
	if !dotestEqual(ids, []string{"SHA256:aaaa", "SHA256:bbbb", "SHA256:cccc"}) {
		t.Errorf("merged bindings = %v, want the sorted union", ids)
	}
	if a.Email != "nobody@example.invalid" || a.TeamName != "Test Team" {
		t.Errorf("merge did not pick up the account details: %+v", a)
	}

	// Merging twice must not change anything, or the stored file churns.
	first := fmt.Sprint(a.Keys)
	a.Merge(b)
	if fmt.Sprint(a.Keys) != first {
		t.Errorf("merging twice changed the bindings: %v then %v", first, a.Keys)
	}

	if s := a.String(); !strings.Contains(s, "account-uuid") || !strings.Contains(s, "Test Team") {
		t.Errorf("String() = %s, want it to mention the account and team", s)
	}
}

func Test_DODropletAccountMerge(t *testing.T) {
	dotestQuiet(t)

	a := NewDODropletAccount("do:testaccount", 900006, "a-droplet", "198.51.100.6", []data.KeyBindingImpl{
		{KeyID: "SHA256:aaaa", Location: data.INSTANCE_ROOT_CREDENTIALS},
	})
	b := NewDODropletAccount("do:testaccount", 900006, "a-droplet", "198.51.100.7", []data.KeyBindingImpl{
		{KeyID: "SHA256:bbbb", Location: data.INSTANCE_ROOT_CREDENTIALS},
	})

	a.Merge(b)

	if len(a.Keys) != 2 {
		t.Errorf("merged bindings = %v, want both", a.Keys)
	}
	if a.PublicIPv4 != "198.51.100.7" {
		t.Errorf("address = %s, want the newly fetched one", a.PublicIPv4)
	}

	// A mismatched type is a bug elsewhere, but it must be reported rather
	// than panic.
	printed := dotestCaptureOutput(t, func() {
		a.Merge(NewDOAccount("do:testaccount", "account-uuid", nil))
	})
	if !strings.Contains(printed, "non droplet account") {
		t.Errorf("merging a mismatched account type reported %q, want a complaint", printed)
	}
	if len(a.Keys) != 2 {
		t.Errorf("a mismatched merge changed the bindings: %v", a.Keys)
	}
}

// Test_DOAccountsRoundTrip covers what the library needs from a stored object:
// a Type field naming the registered type, identifiers to cache it under, and
// a JSON form that survives a round trip.
func Test_DOAccountsRoundTrip(t *testing.T) {
	dotestQuiet(t)

	account := NewDOAccount("do:testaccount", "account-uuid", []data.KeyBindingImpl{
		{KeyID: "SHA256:aaaa", Name: "a-key"},
	})
	account.Email = "nobody@example.invalid"

	droplet := NewDODropletAccount("do:testaccount", 900007, "a-droplet", "198.51.100.8", []data.KeyBindingImpl{
		{KeyID: "SHA256:aaaa", Location: data.INSTANCE_ROOT_CREDENTIALS},
	})

	if account.Type != reflect.TypeOf(DOAccount{}).Name() {
		t.Errorf("Type field = %s, want %s", account.Type, reflect.TypeOf(DOAccount{}).Name())
	}
	if droplet.Type != reflect.TypeOf(DODropletAccount{}).Name() {
		t.Errorf("Type field = %s, want %s", droplet.Type, reflect.TypeOf(DODropletAccount{}).Name())
	}

	if ids := account.Identifiers(); !dotestEqual(dotestIDs(ids), []string{"account-uuid"}) {
		t.Errorf("account identifiers = %v, want [account-uuid]", ids)
	}
	if ids := droplet.Identifiers(); !dotestEqual(dotestIDs(ids), []string{"do-droplet-900007"}) {
		t.Errorf("droplet identifiers = %v, want [do-droplet-900007]", ids)
	}

	if bindings := dotestBindings(droplet); len(bindings) != 1 || bindings[0].KeyID != "SHA256:aaaa" {
		t.Errorf("droplet bindings = %v, want the one binding", bindings)
	}

	t.Run("account json", func(t *testing.T) {
		bytes, err := json.Marshal(account)
		if err != nil {
			t.Fatal("could not marshal:", err)
		}
		back := new(DOAccount)
		if err := json.Unmarshal(bytes, back); err != nil {
			t.Fatal("could not unmarshal:", err)
		}
		if !reflect.DeepEqual(account, back) {
			t.Errorf("round trip gave %+v, want %+v", back, account)
		}
	})

	t.Run("droplet json", func(t *testing.T) {
		bytes, err := json.Marshal(droplet)
		if err != nil {
			t.Fatal("could not marshal:", err)
		}
		back := new(DODropletAccount)
		if err := json.Unmarshal(bytes, back); err != nil {
			t.Fatal("could not unmarshal:", err)
		}
		if !reflect.DeepEqual(droplet, back) {
			t.Errorf("round trip gave %+v, want %+v", back, droplet)
		}
	})

	t.Run("account merged with the wrong type", func(t *testing.T) {
		printed := dotestCaptureOutput(t, func() {
			account.Merge(droplet)
		})
		if !strings.Contains(printed, "non Digital Ocean account") {
			t.Errorf("merging a mismatched account type reported %q, want a complaint", printed)
		}
		if len(account.Keys) != 1 {
			t.Errorf("a mismatched merge changed the bindings: %v", account.Keys)
		}
	})
}

func Test_DOConnectionIdentity(t *testing.T) {
	d := dotestConn()

	if d.Id() != "do:testaccount" {
		t.Errorf("Id() = %s, want do:testaccount", d.Id())
	}
	if d.String() != "do://testaccount" {
		t.Errorf("String() = %s, want do://testaccount", d.String())
	}

	// The connection must satisfy Connection but deliberately not Changer:
	// this connection is read only.
	var _ Connection = d
	if _, ok := interface{}(d).(Changer); ok {
		t.Error("DOConnection implements Changer; it must remain read only")
	}
}

func dotestInts(in []int) []string {
	out := make([]string, 0, len(in))
	for _, i := range in {
		out = append(out, fmt.Sprint(i))
	}
	return out
}

func dotestIDs(in []data.ID) []string {
	out := make([]string, 0, len(in))
	for _, id := range in {
		out = append(out, string(id))
	}
	return out
}
