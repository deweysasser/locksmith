package connection

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

// ---------------------------------------------------------------------------
// stub HTTP layer
//
// Nothing here opens a socket: the stub answers from canned responses and
// records what it was asked for.  A GitHubConnection built by ghtestConn
// always carries a stub, so no test in this file can reach api.github.com.
// ---------------------------------------------------------------------------

// ghtestResponse is one canned answer.
type ghtestResponse struct {
	status  int
	body    string
	headers map[string]string
	err     error
}

type ghtestClient struct {
	mu        sync.Mutex
	responses []ghtestResponse
	requested []string
	headers   []http.Header
}

func (c *ghtestClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requested = append(c.requested, req.URL.String())
	c.headers = append(c.headers, req.Header.Clone())

	if len(c.responses) == 0 {
		return nil, fmt.Errorf("unexpected request for %s", req.URL)
	}

	r := c.responses[0]
	c.responses = c.responses[1:]

	if r.err != nil {
		return nil, r.err
	}

	resp := &http.Response{
		StatusCode: r.status,
		Status:     fmt.Sprintf("%d %s", r.status, http.StatusText(r.status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(r.body)),
	}
	for k, v := range r.headers {
		resp.Header.Set(k, v)
	}

	return resp, nil
}

func (c *ghtestClient) urls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.requested...)
}

// ghtestConn returns a connection wired to a stub that will give the supplied
// responses, in order.
func ghtestConn(user string, responses ...ghtestResponse) (*GitHubConnection, *ghtestClient) {
	client := &ghtestClient{responses: responses}
	return &GitHubConnection{Type: "GitHubConnection", User: user, client: client}, client
}

func ghtestOK(body string) ghtestResponse {
	return ghtestResponse{status: http.StatusOK, body: body}
}

// ghtestQuiet silences everything but output.Error, which ignores the level.
func ghtestQuiet(t *testing.T) {
	t.Helper()
	old := output.Level
	output.Level = output.ErrorLevel
	t.Cleanup(func() { output.Level = old })
}

// ghtestCollect runs a Fetch and drains both channels, failing the test if the
// fetch does not finish promptly.
func ghtestCollect(t *testing.T, c *GitHubConnection) ([]data.Key, []data.Account) {
	t.Helper()

	keys, accounts := c.Fetch()

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
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Fetch did not close its channels")
	}

	return gotKeys, gotAccounts
}

// ---------------------------------------------------------------------------
// key material
//
// Throwaway keys generated for this test suite.  The fingerprints were taken
// from ssh-keygen -lf, so they are an independent check that the key material
// survives the trip through data.NewKey unaltered.
// ---------------------------------------------------------------------------

const (
	ghtestEd25519   = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEweGM+v1z0u+gGYNfN7seH+m22iG17hHKKFmPzDX6NF"
	ghtestEd25519FP = "SHA256:oEpfNJFV/b0dINR0FAp15QYdMfvdiw/2XYBn52UuDgo"
	// ssh-keygen -E md5 prints this with an "MD5:" prefix; data stores the
	// bare hex form.
	ghtestEd25519MD5FP = "bf:1d:47:e9:94:35:1c:fa:bd:e4:db:15:98:07:f9:f3"

	ghtestRSA   = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDTP0gPA0VFS93VDZ/3la7pIsD7zT5E2Qj22Zy5vDrQelVSvZkgxd+to5kzLUmUSpPbr6wff8tY2T52r09N3aHhoNW9MeNAwIrtUYlTwXQDrPf2OES4VheFerKZj4xX1n7Jkvo+RzrW2rM3opUMga2i5GUqo7fGvxHJl149/TqbK1nQuKYCiwm32/IiF9uvu+k+xnNisi/XxsDVHADLf6ASn8K80XG84nwXuE8r1rbTWUN4mMKaidbwKKytwyzFZFKdOamS5Kbny9Tv+GATh3kVKASqujk2jF7CussJIwkiH6/6uzzrOOc3SOvKppZGucPjyuqNs3v9MK7rYr9TMpYJ"
	ghtestRSAFP = "SHA256:R50lbIvEI36TgV3fQTEwH0URsAFzaXEtJVg53ndOrug"
)

// ghtestBody renders a listing in the exact shape the API returns: objects
// with an id and a key, and nothing else.
func ghtestBody(entries ...[2]string) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf(`{"id":%s,"key":%q}`, e[0], e[1]))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func ghtestIDs(key data.Key) map[data.ID]bool {
	ids := make(map[data.ID]bool)
	for _, id := range key.Identifiers() {
		ids[id] = true
	}
	return ids
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestGitHubFetchReturnsKeysAndAnAccount(t *testing.T) {
	ghtestQuiet(t)

	body := ghtestBody(
		[2]string{"49135991", ghtestEd25519},
		[2]string{"49135992", ghtestRSA},
	)

	c, client := ghtestConn("deweysasser", ghtestOK(body))

	keys, accounts := ghtestCollect(t, c)

	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2", len(keys))
	}

	// The fingerprints must be the ones ssh-keygen computes for this material.
	wantFPs := map[data.ID]string{
		data.ID(ghtestEd25519FP): "gh:deweysasser/49135991",
		data.ID(ghtestRSAFP):     "gh:deweysasser/49135992",
	}

	seen := make(map[data.ID]data.Key)
	for _, k := range keys {
		seen[k.Id()] = k
	}

	for fp, name := range wantFPs {
		k, ok := seen[fp]
		if !ok {
			t.Errorf("no key with fingerprint %s; got %v", fp, keysIDs(keys))
			continue
		}
		gotNames := k.GetNames()
		if names := gotNames.StringArray(); len(names) != 1 || names[0] != name {
			t.Errorf("key %s named %v, want [%s]", fp, names, name)
		}
		if k.IsDeprecated() {
			t.Errorf("key %s came back deprecated", fp)
		}
	}

	// The legacy MD5 fingerprint is one of the identifiers a key is stored
	// under, so a key fetched here matches the same key found elsewhere.
	if k, ok := seen[data.ID(ghtestEd25519FP)]; ok {
		if !ghtestIDs(k)[data.ID(ghtestEd25519MD5FP)] {
			t.Errorf("ed25519 key identifiers %v lack the MD5 fingerprint %s",
				k.Identifiers(), ghtestEd25519MD5FP)
		}
	}

	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accounts))
	}

	acct := accounts[0]
	if acct.Id() != data.ID("deweysasser@github.com") {
		t.Errorf("account id = %q, want %q", acct.Id(), "deweysasser@github.com")
	}
	if acct.ConnectionID() != c.Id() {
		t.Errorf("account connection = %q, want %q", acct.ConnectionID(), c.Id())
	}

	bound := make(map[data.ID]data.KeyBindingImpl)
	for b := range acct.Bindings() {
		bound[b.KeyID] = b
	}
	if len(bound) != 2 {
		t.Errorf("account has %d bindings, want 2", len(bound))
	}
	for fp, name := range wantFPs {
		b, ok := bound[fp]
		if !ok {
			t.Errorf("account has no binding for %s", fp)
			continue
		}
		if b.Name != name {
			t.Errorf("binding for %s named %q, want %q", fp, b.Name, name)
		}
		if b.Location != data.AUTHORIZED_KEYS {
			t.Errorf("binding for %s at %q, want %q", fp, b.Location, data.AUTHORIZED_KEYS)
		}
	}

	// The request must go to the public, unauthenticated endpoint, and must
	// carry no credentials.
	urls := client.urls()
	if len(urls) != 1 {
		t.Fatalf("made %d requests, want 1: %v", len(urls), urls)
	}
	if !strings.HasPrefix(urls[0], GitHubAPIBase+"/users/deweysasser/keys") {
		t.Errorf("requested %q, want the /users/deweysasser/keys endpoint", urls[0])
	}
	if auth := client.headers[0].Get("Authorization"); auth != "" {
		t.Errorf("request carried an Authorization header (%q)", auth)
	}
}

// A user with no keys is not an error: the account still exists, with nothing
// bound to it.
func TestGitHubFetchEmptyKeyList(t *testing.T) {
	ghtestQuiet(t)

	c, _ := ghtestConn("nokeys", ghtestOK("[]"))

	keys, accounts := ghtestCollect(t, c)

	if len(keys) != 0 {
		t.Errorf("got %d keys, want none", len(keys))
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accounts))
	}
	for b := range accounts[0].Bindings() {
		t.Errorf("unexpected binding %v", b)
	}
}

// A 404 means there is no such user.  No account is invented for a user that
// does not exist.
func TestGitHubFetchNoSuchUser(t *testing.T) {
	ghtestQuiet(t)

	c, _ := ghtestConn("nobody", ghtestResponse{
		status: http.StatusNotFound,
		body:   `{"message":"Not Found"}`,
	})

	keys, accounts := ghtestCollect(t, c)

	if len(keys) != 0 || len(accounts) != 0 {
		t.Errorf("got %d keys and %d accounts, want none of either", len(keys), len(accounts))
	}

	_, err := c.fetchKeyEntries()
	if err == nil {
		t.Fatal("fetchKeyEntries succeeded on a 404")
	}
	if !strings.Contains(err.Error(), "nobody") {
		t.Errorf("error %q does not name the user", err)
	}
}

// The failure an unauthenticated caller actually meets: the shared per-IP
// budget is spent.  The message must say so, and say when it recovers, rather
// than reporting a bare 403.
func TestGitHubFetchRateLimited(t *testing.T) {
	ghtestQuiet(t)

	reset := time.Now().Add(37 * time.Minute).Truncate(time.Second)

	c, _ := ghtestConn("deweysasser", ghtestResponse{
		status: http.StatusForbidden,
		body:   `{"message":"API rate limit exceeded for 203.0.113.9."}`,
		headers: map[string]string{
			"X-RateLimit-Limit":     "60",
			"X-RateLimit-Remaining": "0",
			"X-RateLimit-Reset":     fmt.Sprintf("%d", reset.Unix()),
		},
	})

	_, err := c.fetchKeyEntries()
	if err == nil {
		t.Fatal("fetchKeyEntries succeeded on a rate-limited 403")
	}

	msg := err.Error()
	if !strings.Contains(msg, "rate limit") {
		t.Errorf("error %q does not mention the rate limit", msg)
	}
	if !strings.Contains(msg, "60") {
		t.Errorf("error %q does not report the limit", msg)
	}
	if want := reset.Format(time.RFC3339); !strings.Contains(msg, want) {
		t.Errorf("error %q does not report the reset time %s", msg, want)
	}

	// And a 403 that is not a rate limit is reported as itself.
	c2, _ := ghtestConn("deweysasser", ghtestResponse{
		status:  http.StatusForbidden,
		body:    `{"message":"Forbidden"}`,
		headers: map[string]string{"X-RateLimit-Remaining": "59"},
	})

	_, err = c2.fetchKeyEntries()
	if err == nil {
		t.Fatal("fetchKeyEntries succeeded on a plain 403")
	}
	if strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error %q blames the rate limit for a plain 403", err)
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error %q does not report the status", err)
	}
}

// Any other non-200 is reported with its status rather than being parsed.
func TestGitHubFetchServerError(t *testing.T) {
	ghtestQuiet(t)

	c, _ := ghtestConn("deweysasser", ghtestResponse{
		status: http.StatusInternalServerError,
		body:   "boom",
	})

	if _, err := c.fetchKeyEntries(); err == nil {
		t.Fatal("fetchKeyEntries succeeded on a 500")
	} else if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q does not report the status", err)
	}
}

// A transport-level failure (DNS, TLS, timeout) is an error, not a panic.
func TestGitHubFetchTransportError(t *testing.T) {
	ghtestQuiet(t)

	c, _ := ghtestConn("deweysasser", ghtestResponse{err: errors.New("dial tcp: no route to host")})

	if _, err := c.fetchKeyEntries(); err == nil {
		t.Fatal("fetchKeyEntries succeeded when the transport failed")
	} else if !strings.Contains(err.Error(), "no route to host") {
		t.Errorf("error %q loses the underlying cause", err)
	}
}

// A body that is not the JSON we expect is an error, not a partial result.
func TestGitHubFetchMalformedJSON(t *testing.T) {
	ghtestQuiet(t)

	for name, body := range map[string]string{
		"truncated": `[{"id":1,"key":"ssh-rsa AAAA"`,
		"notArray":  `{"message":"Not Found"}`,
		"html":      "<html><body>nope</body></html>",
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := ghtestConn("deweysasser", ghtestOK(body))

			if entries, err := c.fetchKeyEntries(); err == nil {
				t.Fatalf("fetchKeyEntries accepted %s body, returning %v", name, entries)
			}

			keys, accounts := ghtestCollect(t, c)
			if len(keys) != 0 || len(accounts) != 0 {
				t.Errorf("got %d keys and %d accounts from a %s body, want none",
					len(keys), len(accounts), name)
			}
		})
	}
}

// One unparseable key must not cost us the others, and must not leave a
// dangling binding on the account.
func TestGitHubFetchSkipsUnparseableKey(t *testing.T) {
	ghtestQuiet(t)

	body := ghtestBody(
		[2]string{"1", "ssh-rsa this-is-not-base64"},
		[2]string{"2", ghtestEd25519},
		[2]string{"3", ""},
	)

	c, _ := ghtestConn("deweysasser", ghtestOK(body))

	keys, accounts := ghtestCollect(t, c)

	if len(keys) != 1 {
		t.Fatalf("got %d keys (%v), want only the good one", len(keys), keysIDs(keys))
	}
	if keys[0].Id() != data.ID(ghtestEd25519FP) {
		t.Errorf("key id = %q, want %q", keys[0].Id(), ghtestEd25519FP)
	}

	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accounts))
	}
	var bindings []data.KeyBindingImpl
	for b := range accounts[0].Bindings() {
		bindings = append(bindings, b)
	}
	if len(bindings) != 1 {
		t.Fatalf("account has %d bindings, want 1: %v", len(bindings), bindings)
	}
	if bindings[0].KeyID != data.ID(ghtestEd25519FP) {
		t.Errorf("binding is for %q, want %q", bindings[0].KeyID, ghtestEd25519FP)
	}
}

// A user with more keys than fit on a page: every page is followed, and the
// keys of all of them come back.
func TestGitHubFetchFollowsPagination(t *testing.T) {
	ghtestQuiet(t)

	page2 := GitHubAPIBase + "/users/deweysasser/keys?per_page=100&page=2"

	c, client := ghtestConn("deweysasser",
		ghtestResponse{
			status:  http.StatusOK,
			body:    ghtestBody([2]string{"1", ghtestEd25519}),
			headers: map[string]string{"Link": fmt.Sprintf(`<%s>; rel="next", <%s>; rel="last"`, page2, page2)},
		},
		ghtestOK(ghtestBody([2]string{"2", ghtestRSA})),
	)

	keys, _ := ghtestCollect(t, c)

	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2 (one from each page)", len(keys))
	}

	urls := client.urls()
	if len(urls) != 2 {
		t.Fatalf("made %d requests, want 2: %v", len(urls), urls)
	}
	if urls[1] != page2 {
		t.Errorf("second request went to %q, want %q", urls[1], page2)
	}
}

// A "next" link pointing at another host is not followed: it would send the
// next request wherever the response asked.
func TestGitHubFetchIgnoresOffsiteNextLink(t *testing.T) {
	ghtestQuiet(t)

	c, client := ghtestConn("deweysasser", ghtestResponse{
		status: http.StatusOK,
		body:   ghtestBody([2]string{"1", ghtestEd25519}),
		headers: map[string]string{
			"Link": `<https://evil.example.com/users/deweysasser/keys?page=2>; rel="next"`,
		},
	})

	keys, _ := ghtestCollect(t, c)

	if len(keys) != 1 {
		t.Errorf("got %d keys, want 1", len(keys))
	}
	if urls := client.urls(); len(urls) != 1 {
		t.Errorf("made %d requests, want 1: %v", len(urls), urls)
	}
}

func TestGitHubIdAndString(t *testing.T) {
	c := &GitHubConnection{Type: "GitHubConnection", User: "deweysasser"}

	if got, want := c.Id(), data.ID("gh:deweysasser"); got != want {
		t.Errorf("Id() = %q, want %q", got, want)
	}
	if got, want := c.String(), "github://deweysasser"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	// Id and String depend on the user alone, so a connection read back from
	// the library keys to the same place as the one that was stored.
	other := &GitHubConnection{Type: "GitHubConnection", User: "deweysasser"}
	if other.Id() != c.Id() || other.String() != c.String() {
		t.Error("two connections for the same user do not agree on Id/String")
	}

	if different := (&GitHubConnection{Type: "GitHubConnection", User: "someone"}); different.Id() == c.Id() {
		t.Error("connections for different users share an Id")
	}

	// A GitHubConnection is a Connection but deliberately not a Changer.
	var _ Connection = c
	if _, ok := interface{}(c).(Changer); ok {
		t.Error("GitHubConnection implements Changer; GitHub allows no writes to another user's keys")
	}
}

// A user name with characters that mean something in a URL is escaped into
// the path rather than changing where the request goes.
func TestGitHubUserIsEscapedIntoThePath(t *testing.T) {
	ghtestQuiet(t)

	c, client := ghtestConn("../../evil?x=1", ghtestOK("[]"))

	if _, err := c.fetchKeyEntries(); err != nil {
		t.Fatalf("fetchKeyEntries: %v", err)
	}

	urls := client.urls()
	if len(urls) != 1 {
		t.Fatalf("made %d requests, want 1: %v", len(urls), urls)
	}
	if !strings.HasPrefix(urls[0], GitHubAPIBase+"/users/") {
		t.Errorf("request went to %q, outside the /users/ path", urls[0])
	}
}

// The default client is a real HTTP client with a timeout -- a connection
// deserialized from the library has no client of its own, and must not be
// left with a nil one or an unbounded wait.
func TestGitHubDefaultClient(t *testing.T) {
	c := &GitHubConnection{Type: "GitHubConnection", User: "deweysasser"}

	client, ok := c.httpClient().(*http.Client)
	if !ok {
		t.Fatalf("default client is %T, want *http.Client", c.httpClient())
	}
	if client.Timeout <= 0 {
		t.Error("default client has no timeout")
	}
}

func keysIDs(keys []data.Key) []data.ID {
	ids := make([]data.ID, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, k.Id())
	}
	return ids
}
