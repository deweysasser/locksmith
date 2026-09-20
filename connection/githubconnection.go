package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

// GitHubAPIBase is the root of the GitHub REST API.  Only
// /users/{user}/keys is used, and that endpoint is public: no token is
// needed, and none is ever sent.
const GitHubAPIBase = "https://api.github.com"

const (
	// githubPerPage is the page size asked for.  100 is the API maximum.
	githubPerPage = 100

	// githubMaxPages bounds the Link-header walk so a server that keeps
	// handing back a "next" link cannot loop forever.
	githubMaxPages = 100

	// githubMaxBody caps how much of a response is read.  A key listing is
	// a few KB; anything on this scale is a sign we are not talking to the
	// API we think we are.
	githubMaxBody = 8 << 20
)

// githubHTTP is the part of *http.Client this connection uses.  Taking an
// interface (rather than the concrete client) is what lets the tests answer
// requests without a network, a listener or a transport.
type githubHTTP interface {
	Do(req *http.Request) (*http.Response, error)
}

// githubDefaultClient is used whenever a connection has no client of its own,
// which is every connection read back from the library: the client is not a
// persisted field.
var githubDefaultClient githubHTTP = &http.Client{Timeout: 30 * time.Second}

// GitHubConnection reads the public SSH keys a named GitHub user has
// published, via GET /users/{user}/keys.
//
// It is deliberately read-only and does not implement Changer.  GitHub only
// permits writing keys on the *authenticated* user's own account
// (POST/DELETE /user/keys), never on an arbitrary user's, so there is no
// remote change this connection could honestly apply.  `apply` skips a
// connection that is not a Changer with a warning, which is the behaviour we
// want here.
type GitHubConnection struct {
	// Type is the persisted discriminator; see lib.AddType.  It must be set
	// at every construction site -- the library does not fill it in.
	Type string

	// User is the GitHub login whose keys are catalogued.
	User string

	// client, when set, replaces githubDefaultClient.  It is unexported and
	// so never serialized.
	client githubHTTP
}

func (c *GitHubConnection) String() string {
	return "github://" + c.User
}

func (c *GitHubConnection) Id() data.ID {
	return data.ID("gh:" + c.User)
}

func (c *GitHubConnection) httpClient() githubHTTP {
	if c.client != nil {
		return c.client
	}
	return githubDefaultClient
}

// githubKeyEntry is one element of the /users/{user}/keys response.  The
// endpoint returns exactly these two fields -- no title, no comment, no
// creation date -- so the numeric ID and the login are all we have to label a
// key with.
type githubKeyEntry struct {
	ID  int64  `json:"id"`
	Key string `json:"key"`
}

func (c *GitHubConnection) Fetch(ctx context.Context) (keys <-chan data.Key, accounts <-chan data.Account) {
	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	go func() {
		defer close(cKeys)
		defer close(cAccounts)
		c.fetch(ctx, cKeys, cAccounts)
	}()

	return cKeys, cAccounts
}

// fetch does the work of Fetch against caller-supplied channels, so that it
// can be tested without the channel plumbing.  It does not close them.
//
// A GitHub user is modelled as an account with the keys bound to it, rather
// than as a bare pile of keys the way FileConnection does it.  A file is a
// place keys are *stored*; a GitHub user is a place keys are *authorized* --
// the same thing SSHHostConnection and AWSConnection report accounts for --
// and it is the thing `plan` would want to name when it decides a published
// key should go away.  The account is a data.SSHAccount because these are the
// SSH keys of a login at github.com, and because a dedicated account type
// would mean adding one to data/.
func (c *GitHubConnection) fetch(ctx context.Context, keys chan<- data.Key, accounts chan<- data.Account) {
	output.Debug("Fetching keys for GitHub user", c.User)

	entries, err := c.fetchKeyEntries(ctx)
	if err != nil {
		output.Error(c.String()+":", err)
		return
	}

	// The API reports no creation date, so record when we first saw the key.
	// data.Key.Merge keeps the earliest of the two, so a key already known
	// from somewhere older keeps that older date.
	now := time.Now()

	bindings := make([]data.KeyBindingImpl, 0, len(entries))

	for _, entry := range entries {
		name := c.keyName(entry)

		key := data.NewKey(entry.Key, now, name)
		if key == nil {
			output.Warn(c.String()+":", "could not parse key", name)
			continue
		}

		output.Debug("Found GitHub key", name, key.Id())

		keys <- key
		bindings = append(bindings, data.KeyBindingImpl{
			KeyID:    key.Id(),
			Location: data.AUTHORIZED_KEYS,
			Name:     name,
		})
	}

	account := data.NewSSHAccount(c.User, c.User+"@github.com", c.Id(), bindings)
	// The endpoint returns the user's complete published key set, so a key
	// recorded here and no longer listed has genuinely been removed.
	account.MarkObserved(data.AUTHORIZED_KEYS)
	accounts <- account
}

// keyName labels a key with both the login and GitHub's numeric key id.  Both
// halves matter: the login is what a user will filter on, and the id is the
// only thing that distinguishes two keys of the same user in the listing.
func (c *GitHubConnection) keyName(entry githubKeyEntry) string {
	return fmt.Sprintf("gh:%s/%d", c.User, entry.ID)
}

// fetchKeyEntries retrieves every page of the user's key listing.
func (c *GitHubConnection) fetchKeyEntries(ctx context.Context) ([]githubKeyEntry, error) {
	next := fmt.Sprintf("%s/users/%s/keys?per_page=%d",
		GitHubAPIBase, url.PathEscape(c.User), githubPerPage)

	var all []githubKeyEntry

	for page := 0; next != "" && page < githubMaxPages; page++ {
		entries, link, err := c.fetchPage(ctx, next)
		if err != nil {
			return nil, err
		}

		all = append(all, entries...)
		next = link
	}

	return all, nil
}

// fetchPage requests one page and returns its entries plus the URL of the
// next page, if the response advertised one.
func (c *GitHubConnection) fetchPage(ctx context.Context, u string) (entries []githubKeyEntry, next string, err error) {
	// WithContext, not NewRequest: the client timeout bounds a slow response,
	// but only the context aborts a request already in flight when the run is
	// cancelled.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	var body []byte
	if resp.Body != nil {
		if body, err = io.ReadAll(io.LimitReader(resp.Body, githubMaxBody)); err != nil {
			return nil, "", fmt.Errorf("failed to read response from %s: %w", u, err)
		}
	}

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, "", fmt.Errorf("no such GitHub user %q", c.User)
	case resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusTooManyRequests:
		return nil, "", githubForbiddenError(resp)
	case resp.StatusCode != http.StatusOK:
		return nil, "", fmt.Errorf("GET %s returned %s", u, resp.Status)
	}

	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, "", fmt.Errorf("failed to parse response from %s: %w", u, err)
	}

	return entries, githubNextLink(resp, u), nil
}

// githubForbiddenError explains a 403/429.  Unauthenticated callers share a
// per-IP budget of 60 requests an hour, so this is the failure a user is most
// likely to meet, and "403 Forbidden" on its own would send them looking for a
// permissions problem that does not exist.
func githubForbiddenError(resp *http.Response) error {
	if resp.Header.Get("X-RateLimit-Remaining") != "0" {
		return fmt.Errorf("GitHub refused the request (%s)", resp.Status)
	}

	limit := resp.Header.Get("X-RateLimit-Limit")
	if limit == "" {
		limit = "unknown"
	}

	reset := "an unknown time"
	if secs, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		reset = time.Unix(secs, 0).Format(time.RFC3339)
	}

	return fmt.Errorf("GitHub rate limit of %s requests exhausted (%s); it resets at %s",
		limit, resp.Status, reset)
}

// githubNextLink returns the rel="next" URL from a Link header, or "" when
// there is no further page.  Anything pointing somewhere other than the host
// we just asked is ignored: following it would send the next request to a
// server of the response's choosing.
func githubNextLink(resp *http.Response, current string) string {
	header := resp.Header.Get("Link")
	if header == "" {
		return ""
	}

	base, err := url.Parse(current)
	if err != nil {
		return ""
	}

	for _, section := range strings.Split(header, ",") {
		parts := strings.Split(section, ";")
		if len(parts) < 2 {
			continue
		}

		isNext := false
		for _, param := range parts[1:] {
			p := strings.TrimSpace(param)
			if p == `rel="next"` || p == "rel=next" {
				isNext = true
			}
		}
		if !isNext {
			continue
		}

		raw := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(raw, "<") || !strings.HasSuffix(raw, ">") {
			continue
		}

		u, err := url.Parse(raw[1 : len(raw)-1])
		if err != nil || u.Scheme != base.Scheme || u.Host != base.Host {
			continue
		}

		return u.String()
	}

	return ""
}
