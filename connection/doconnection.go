package connection

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"github.com/digitalocean/godo"
)

// DOTokenVariable is the environment variable from which the Digital Ocean API
// token is read.  It is the name godo's own users and terraform both use.
const DOTokenVariable = "DIGITALOCEAN_ACCESS_TOKEN"

// doPageSize is the largest page Digital Ocean will return.
const doPageSize = 200

// DOConnection surveys a single Digital Ocean account: the SSH keys registered
// on the account and the droplets running in it.  It is read only -- it
// deliberately does not implement Changer, and every call it makes is a GET.
//
// Name is a label for the account this connection refers to; the credential
// itself always comes from the DIGITALOCEAN_ACCESS_TOKEN environment variable.
type DOConnection struct {
	Type, Name string
}

func (d *DOConnection) String() string {
	return fmt.Sprintf("do://%s", d.Name)
}

func (d *DOConnection) Id() data.ID {
	// Prefixed so that a Digital Ocean connection named, say, "default" cannot
	// collide with an AWS profile of the same name.
	return data.ID("do:" + d.Name)
}

// DOAccount is the Digital Ocean account (or team) a token belongs to.  The
// account's registered SSH keys are bound to it, which is where the keys
// Digital Ocean holds on the user's behalf actually live.
type DOAccount struct {
	Type       string
	Connection data.ID
	Keys       []data.KeyBindingImpl
	AccountID  string
	Email      string `json:",omitempty"`
	TeamName   string `json:",omitempty"`
}

func NewDOAccount(connID data.ID, accountID string, keys []data.KeyBindingImpl) *DOAccount {
	if keys == nil {
		keys = []data.KeyBindingImpl{}
	}
	return &DOAccount{
		Type:       "DOAccount",
		Connection: connID,
		Keys:       keys,
		AccountID:  accountID,
	}
}

func (a *DOAccount) Id() data.ID {
	return data.ID(a.AccountID)
}

func (a *DOAccount) Identifiers() []data.ID {
	return []data.ID{a.Id()}
}

func (a *DOAccount) ConnectionID() data.ID {
	return a.Connection
}

func (a *DOAccount) Bindings() <-chan data.KeyBindingImpl {
	return doBindingChannel(a.Keys)
}

func (a *DOAccount) Merge(account data.Account) {
	other, ok := account.(*DOAccount)
	if !ok {
		output.Error(a, "asked to merge with non Digital Ocean account", account)
		return
	}

	a.Keys = doMergeBindings(a.Keys, other.Keys)
	if a.Email == "" {
		a.Email = other.Email
	}
	if a.TeamName == "" {
		a.TeamName = other.TeamName
	}
}

func (a *DOAccount) String() string {
	var parts []string
	if a.Email != "" {
		parts = append(parts, a.Email)
	}
	if a.TeamName != "" {
		parts = append(parts, a.TeamName)
	}

	if len(parts) == 0 {
		return fmt.Sprintf("do %s", a.AccountID)
	}

	return fmt.Sprintf("do %s (%s)", a.AccountID, strings.Join(parts, ", "))
}

// DODropletAccount is a single droplet, analogous to AWSInstanceAccount.
type DODropletAccount struct {
	Type       string
	Connection data.ID
	Keys       []data.KeyBindingImpl
	DropletID  int
	Name       string `json:",omitempty"`
	PublicIPv4 string `json:",omitempty"`
}

func NewDODropletAccount(connID data.ID, dropletID int, name, publicIPv4 string, keys []data.KeyBindingImpl) *DODropletAccount {
	if keys == nil {
		keys = []data.KeyBindingImpl{}
	}
	return &DODropletAccount{
		Type:       "DODropletAccount",
		Connection: connID,
		Keys:       keys,
		DropletID:  dropletID,
		Name:       name,
		PublicIPv4: publicIPv4,
	}
}

func (a *DODropletAccount) Id() data.ID {
	return data.ID(fmt.Sprintf("do-droplet-%d", a.DropletID))
}

func (a *DODropletAccount) Identifiers() []data.ID {
	return []data.ID{a.Id()}
}

func (a *DODropletAccount) ConnectionID() data.ID {
	return a.Connection
}

func (a *DODropletAccount) Bindings() <-chan data.KeyBindingImpl {
	return doBindingChannel(a.Keys)
}

func (a *DODropletAccount) Merge(account data.Account) {
	other, ok := account.(*DODropletAccount)
	if !ok {
		output.Error(a, "asked to merge with non droplet account", account)
		return
	}

	a.Keys = doMergeBindings(a.Keys, other.Keys)
	if other.Name != "" {
		a.Name = other.Name
	}
	// The address can move between fetches, so the newest one wins.
	a.PublicIPv4 = other.PublicIPv4
}

func (a *DODropletAccount) String() string {
	s := string(a.Id())

	var parts []string
	if a.Name != "" {
		parts = append(parts, a.Name)
	}
	if a.PublicIPv4 != "" {
		parts = append(parts, a.PublicIPv4)
	}

	if len(parts) == 0 {
		return s
	}

	return fmt.Sprintf("%s (%s)", s, strings.Join(parts, ", "))
}

func doBindingChannel(bindings []data.KeyBindingImpl) <-chan data.KeyBindingImpl {
	c := make(chan data.KeyBindingImpl)

	go func() {
		defer close(c)
		for _, b := range bindings {
			c <- b
		}
	}()

	return c
}

// doMergeBindings returns the union of two binding lists in a stable order.
// Accounts are re-merged and rewritten on every fetch and the repository is
// meant to be kept in git, so the order must not churn.
func doMergeBindings(a, b []data.KeyBindingImpl) []data.KeyBindingImpl {
	seen := make(map[data.KeyBindingImpl]bool, len(a)+len(b))
	merged := make([]data.KeyBindingImpl, 0, len(a)+len(b))

	for _, list := range [][]data.KeyBindingImpl{a, b} {
		for _, binding := range list {
			if seen[binding] {
				continue
			}
			seen[binding] = true
			merged = append(merged, binding)
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		switch {
		case merged[i].KeyID != merged[j].KeyID:
			return merged[i].KeyID < merged[j].KeyID
		case merged[i].Location != merged[j].Location:
			return merged[i].Location < merged[j].Location
		default:
			return merged[i].Name < merged[j].Name
		}
	})

	return merged
}

// Fetch surveys the account's SSH keys and droplets.  Everything it does is a
// read; there is no write path to Digital Ocean anywhere in this file.
func (d *DOConnection) Fetch(ctx context.Context) (keys <-chan data.Key, accounts <-chan data.Account) {
	output.Debug("Fetching from digital ocean", d.Name)

	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	go func() {
		defer close(cKeys)
		defer close(cAccounts)

		client, err := doClient()
		if err != nil {
			output.Error(d.String()+":", err)
			return
		}

		d.fetch(ctx, client.Account, client.Keys, client.Droplets, cKeys, cAccounts)
	}()

	return cKeys, cAccounts
}

// doClient builds a godo client from the token in the environment.  A missing
// token is an error rather than a panic or an anonymous client.
func doClient() (*godo.Client, error) {
	token := strings.TrimSpace(os.Getenv(DOTokenVariable))
	if token == "" {
		return nil, fmt.Errorf("no Digital Ocean API token found; set %s", DOTokenVariable)
	}

	return godo.NewFromToken(token), nil
}

// fetch is the whole survey, expressed in terms of godo's service interfaces so
// that it can be exercised without a network or a credential.
func (d *DOConnection) fetch(ctx context.Context,
	accountSvc godo.AccountService,
	keySvc godo.KeysService,
	dropletSvc godo.DropletsService,
	cKeys chan<- data.Key,
	cAccounts chan<- data.Account) {

	account := d.fetchAccountInfo(ctx, accountSvc)
	keymap := d.fetchKeys(ctx, keySvc, account, cKeys, cAccounts)
	d.fetchDroplets(ctx, dropletSvc, keymap, cAccounts)
}

// fetchAccountInfo identifies the account the token belongs to.  If that lookup
// fails we carry on with an account named after the connection: knowing which
// keys and droplets exist is worth more than knowing the account's UUID.
func (d *DOConnection) fetchAccountInfo(ctx context.Context, svc godo.AccountService) *DOAccount {
	account := NewDOAccount(d.Id(), string(d.Id()), nil)

	info, _, err := svc.Get(ctx)
	if err != nil {
		output.Warn(d.String()+":", "failed to fetch account information:", err)
		return account
	}

	if info == nil {
		output.Warn(d.String() + ": account information was empty")
		return account
	}

	if info.UUID != "" {
		account.AccountID = info.UUID
	}
	account.Email = info.Email
	if info.Team != nil {
		account.TeamName = info.Team.Name
	}

	return account
}

// fetchKeys pushes every SSH key registered on the account, binds them all to
// the account, and returns a map from Digital Ocean key ID to locksmith key ID.
func (d *DOConnection) fetchKeys(ctx context.Context, svc godo.KeysService, account *DOAccount, cKeys chan<- data.Key, cAccounts chan<- data.Account) map[int]data.ID {
	keymap := make(map[int]data.ID)

	opt := &godo.ListOptions{PerPage: doPageSize}

	for {
		page, resp, err := svc.List(ctx, opt)
		if err != nil {
			output.Error(d.String()+":", "failed to list SSH keys:", err)
			break
		}

		for _, k := range page {
			key := d.keyFor(k)
			if key == nil {
				continue
			}

			output.Debug(d, "found ssh key", k.Name)
			cKeys <- key
			keymap[k.ID] = key.Id()
			account.Keys = append(account.Keys, data.KeyBindingImpl{KeyID: key.Id(), Name: k.Name})
		}

		next, more := doNextPage(resp, opt.Page)
		if !more {
			break
		}
		opt.Page = next
	}

	account.Keys = doMergeBindings(account.Keys, nil)
	cAccounts <- account

	return keymap
}

// keyFor converts a Digital Ocean SSH key into a locksmith key.  Digital Ocean
// gives us the public key body, so wherever it parses we get a key with full
// identifiers; if it does not (an ECDSA key, say, which data.NewKey does not
// recognise) we fall back to the MD5 fingerprint Digital Ocean reports, which
// is the same form ssh.FingerprintLegacyMD5 produces and therefore still
// correlates with keys discovered elsewhere.
func (d *DOConnection) keyFor(k godo.Key) data.Key {
	if k.PublicKey != "" {
		if key := data.NewKey(k.PublicKey, time.Time{}, k.Name); key != nil {
			return key
		}
		output.Debug(d, "could not parse public key material for", k.Name, "-- falling back to its fingerprint")
	}

	if k.Fingerprint == "" {
		output.Warn(d.String()+":", "ignoring SSH key", k.Name, "with neither public key nor fingerprint")
		return nil
	}

	return data.NewSSHKeyFromFingerprint(k.Name, time.Time{}, data.ID(k.Fingerprint))
}

// fetchDroplets pushes an account for every droplet in the account.
func (d *DOConnection) fetchDroplets(ctx context.Context, svc godo.DropletsService, keymap map[int]data.ID, cAccounts chan<- data.Account) {
	opt := &godo.ListOptions{PerPage: doPageSize}

	for {
		page, resp, err := svc.List(ctx, opt)
		if err != nil {
			output.Error(d.String()+":", "failed to list droplets:", err)
			return
		}

		for _, droplet := range page {
			account := d.dropletAccount(droplet, keymap)
			output.Debug(d, "found droplet account", account)
			cAccounts <- account
		}

		next, more := doNextPage(resp, opt.Page)
		if !more {
			return
		}
		opt.Page = next
	}
}

func (d *DOConnection) dropletAccount(droplet godo.Droplet, keymap map[int]data.ID) *DODropletAccount {
	ip, err := droplet.PublicIPv4()
	if err != nil {
		output.Debug(d, "droplet", droplet.Name, "has no addresses:", err)
		ip = ""
	}

	bindings := d.dropletBindings(droplet.Name, dropletKeyIDs(droplet), keymap)

	return NewDODropletAccount(d.Id(), droplet.ID, droplet.Name, ip, bindings)
}

// dropletKeyIDs returns the IDs of the SSH keys Digital Ocean reports as having
// been injected into the droplet's root account.
//
// Today that is always empty.  ssh_keys is an argument to droplet *creation*
// only: neither GET /v2/droplets nor GET /v2/droplets/{id} echo it back (both
// verified against a live account), and godo.Droplet has no field for it.  The
// seam is kept here, rather than the binding logic being deleted, because this
// is the one place that would have to change if Digital Ocean ever does report
// it -- and because a droplet's bindings are otherwise unverifiable.
func dropletKeyIDs(_ godo.Droplet) []int {
	return nil
}

// dropletBindings maps Digital Ocean key IDs to bindings on a droplet's root
// account.  A key we have never seen listed is reported and skipped rather than
// recorded as a binding to a key that does not exist.
func (d *DOConnection) dropletBindings(dropletName string, keyIDs []int, keymap map[int]data.ID) []data.KeyBindingImpl {
	bindings := make([]data.KeyBindingImpl, 0, len(keyIDs))

	for _, id := range keyIDs {
		keyID, ok := keymap[id]
		if !ok {
			output.Warn(d.String()+":", "droplet", dropletName, "references SSH key", id, "which is not registered on the account")
			continue
		}

		bindings = append(bindings, data.KeyBindingImpl{
			KeyID:    keyID,
			Location: data.INSTANCE_ROOT_CREDENTIALS,
		})
	}

	return bindings
}

// doNextPage reports the page to ask for next.  Digital Ocean describes
// pagination in the response body rather than in headers, so an absent or
// last-page link set means we are done.  A page number that fails to advance is
// treated as the end of the list so that a malformed response cannot spin.
func doNextPage(resp *godo.Response, current int) (int, bool) {
	if resp == nil || resp.Links == nil || resp.Links.IsLastPage() {
		return 0, false
	}

	page, err := resp.Links.CurrentPage()
	if err != nil {
		output.Debug("could not determine current digital ocean page:", err)
		return 0, false
	}

	if page+1 <= current {
		return 0, false
	}

	return page + 1, true
}
