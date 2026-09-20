package connection

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deweysasser/locksmith/data"
)

// A prologue that makes `getent passwd` answer with a realistic passwd table.
// The "broken" line has too few fields and must be skipped, and the heredoc
// leaves an empty trailing line, which must be skipped too.
const sshtestPasswdPrologue = `getent() {
	cat <<'PASSWD'
root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
broken:x:2
ubuntu:x:1000:1000:Ubuntu,,,:/home/ubuntu:/bin/bash
PASSWD
}
`

func TestRetreiveSystemUsers(t *testing.T) {
	sshtestQuiet(t)
	sshtestUseSandbox(t, sshtestPasswdPrologue)

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}
	defer cmd.Close()

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@host.example.com", Sudo: true}

	homes := make(map[string]string)
	var order []string
	for acct := range c.retreiveSystemUsers(cmd) {
		homes[acct.User] = acct.Home
		order = append(order, acct.User)
	}

	want := map[string]string{
		"root":   "/root",
		"daemon": "/usr/sbin",
		"ubuntu": "/home/ubuntu",
	}

	if len(order) != len(want) {
		t.Errorf("got %d users (%v), want %d -- short and empty lines must be skipped",
			len(order), order, len(want))
	}
	for user, home := range want {
		if got, ok := homes[user]; !ok {
			t.Errorf("user %s was not reported", user)
		} else if got != home {
			t.Errorf("home for %s = %q, want %q", user, got, home)
		}
	}
	if _, ok := homes["broken"]; ok {
		t.Error("the malformed passwd line should have been skipped")
	}
}

// When the remote has no getent at all, the channel simply closes empty rather
// than the fetch blocking or panicking.
func TestRetreiveSystemUsersCommandFails(t *testing.T) {
	sshtestQuiet(t)
	sshtestUseSandbox(t, "getent() { return 2; }")

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}
	defer cmd.Close()

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}

	for acct := range c.retreiveSystemUsers(cmd) {
		t.Errorf("expected no users, got %v", acct)
	}
}

// retrieveKeysFrom sleeps up to 500ms per call, so the tests below keep the
// number of calls small on purpose.
func TestRetrieveKeys(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	other := sshtestGenerateKeyLine(t, "bob@example.com")
	sshtestWriteAuthorizedKeys(t, home,
		"# keys for alice",
		authorizedKey+" alice@example.com",
		"",
		other,
		"   ",
	)

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}
	defer cmd.Close()

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com"}

	keys, err := c.RetrieveKeys(cmd)
	if err != nil {
		t.Fatalf("RetrieveKeys: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2 -- blank and comment lines must be skipped: %v", len(keys), keys)
	}

	comments := make(map[string]bool)
	for _, k := range keys {
		sshKey, ok := k.Key.(*data.SSHKey)
		if !ok {
			t.Fatalf("key is %T, want *data.SSHKey", k.Key)
		}
		for _, c := range sshKey.Comments.StringArray() {
			comments[c] = true
		}
	}
	for _, want := range []string{"alice@example.com", "bob@example.com"} {
		if !comments[want] {
			t.Errorf("no key carried the comment %q; got %v", want, comments)
		}
	}
}

// The sudo path reads another user's authorized_keys by absolute path.
func TestRetrieveKeysForAccount(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	sshtestWriteAuthorizedKeys(t, home, authorizedKey+" alice@example.com")

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}
	defer cmd.Close()

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}

	keys, err := c.retrieveKeysFor(cmd, remoteAccount{User: "alice", Home: home}, "sudo")
	if err != nil {
		t.Fatalf("retrieveKeysFor: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("got %d keys, want 1: %v", len(keys), keys)
	}
}

// An account with no authorized_keys file is normal, not an error.
func TestRetrieveKeysMissingFile(t *testing.T) {
	sshtestQuiet(t)
	sshtestUseSandbox(t, "")

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}
	defer cmd.Close()

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com"}

	keys, err := c.RetrieveKeys(cmd)
	if err == nil {
		t.Error("reading a missing authorized_keys should report an error, not an empty result: that difference is what stops a failed read from clearing the account")
	}
	if len(keys) != 0 {
		t.Errorf("got %d keys for a host with no authorized_keys, want 0", len(keys))
	}
}

// sshtestPasswdFor builds a fake-ssh prologue whose `getent passwd` reports the
// given users with the given home directories.
func sshtestPasswdFor(homes map[string]string) string {
	var b strings.Builder
	b.WriteString("getent() {\n\tcat <<'PASSWD'\n")
	uid := 1000
	for user, home := range homes {
		fmt.Fprintf(&b, "%s:x:%d:%d::%s:/bin/sh\n", user, uid, uid, home)
		uid++
	}
	b.WriteString("PASSWD\n}\n")
	return b.String()
}

// The sudo fetch walks every system account and collects the keys it finds,
// naming each account <user>@<host> regardless of the login used to connect.
func TestSSHHostConnectionFetchSudo(t *testing.T) {
	sshtestQuiet(t)

	base := t.TempDir()
	homes := map[string]string{
		"alice": filepath.Join(base, "alice"),
		"bob":   filepath.Join(base, "bob"),
		"carol": filepath.Join(base, "carol"), // no .ssh at all
	}
	sshtestWriteAuthorizedKeys(t, homes["alice"], authorizedKey+" alice@example.com")
	sshtestWriteAuthorizedKeys(t, homes["bob"], sshtestGenerateKeyLine(t, "bob@example.com"))

	sshtestUseSandbox(t, sshtestPasswdFor(homes))

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@host.example.com", Sudo: true}

	keys, accounts := fetchAll(c)

	if len(keys) != 2 {
		t.Errorf("got %d keys, want 2: %v", len(keys), keys)
	}
	// carol has no keys, so no account is reported for her.
	// Three, not two: carol has no authorized_keys, and a keyless account must
	// still be reported so that bindings previously recorded for it can be
	// cleared.  command.ingestAccounts is what declines to store a *new* empty
	// account.
	if len(accounts) != 3 {
		t.Fatalf("got %d accounts, want 3: %v", len(accounts), accounts)
	}

	names := make(map[string]bool)
	for _, a := range accounts {
		names[fmt.Sprintf("%s", a)] = true
	}
	for _, want := range []string{"alice@host.example.com", "bob@host.example.com"} {
		found := false
		for name := range names {
			if strings.Contains(name, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no account named %q; got %v", want, names)
		}
	}
}

// Without sudo only the login account is examined.
func TestSSHHostConnectionFetchNonSudo(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "whoami() { echo ubuntu; }")

	sshtestWriteAuthorizedKeys(t, home,
		authorizedKey+" alice@example.com",
		sshtestGenerateKeyLine(t, "bob@example.com"),
	)

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@host.example.com"}

	keys, accounts := fetchAll(c)

	if len(keys) != 2 {
		t.Errorf("got %d keys, want 2: %v", len(keys), keys)
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want 1: %v", len(accounts), accounts)
	}
}

// An account with no keys at all produces no account record, and the channels
// still close rather than leaving the caller waiting.
func TestSSHHostConnectionFetchNonSudoNoKeys(t *testing.T) {
	sshtestQuiet(t)
	sshtestUseSandbox(t, "whoami() { echo ubuntu; }")

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@host.example.com"}

	keys, accounts := fetchAll(c)

	if len(keys) != 0 {
		t.Errorf("got %d keys, want none", len(keys))
	}
	// The account IS reported despite having no keys.  That is the whole point:
	// an emptied authorized_keys has to be observable, or bindings recorded for
	// the account could never be cleared.
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want the surveyed account reported: %v", len(accounts), accounts)
	}
	for b := range accounts[0].Bindings() {
		t.Errorf("account should carry no bindings, got %v", b)
	}
}

// A failed read yields no keys, exactly as an empty file does. If the fetch
// claimed completeness on that basis, a transient sudo or permission failure
// would delete every binding recorded for the account. The claim must depend on
// the read having actually succeeded.
func TestSSHFetchDoesNotClaimAuthorityWhenTheReadFails(t *testing.T) {
	sshtestQuiet(t)
	// cat fails for every account, as an unreadable home or a refused sudo
	// would.
	sshtestUseSandbox(t, sshtestPasswdPrologue+`
cat() { case "$*" in *authorized_keys*) return 1;; *) command cat "$@";; esac; }`)

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@host.example.com", Sudo: true}

	_, accounts := fetchAll(c)

	if len(accounts) == 0 {
		t.Fatal("expected the surveyed accounts to be reported")
	}
	for _, a := range accounts {
		if claimed := sshtestObserved(a); len(claimed) != 0 {
			t.Errorf("account %s claimed authority over %v after a failed read", a.Id(), claimed)
		}
	}
}

// The companion: a successful read of a genuinely empty file must claim, or an
// emptied authorized_keys could never clear anything.
func TestSSHFetchClaimsAuthorityOnAnEmptyFile(t *testing.T) {
	sshtestQuiet(t)
	sshtestUseSandbox(t, sshtestPasswdPrologue+`
cat() { case "$*" in *authorized_keys*) return 0;; *) command cat "$@";; esac; }`)

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@host.example.com", Sudo: true}

	_, accounts := fetchAll(c)

	if len(accounts) == 0 {
		t.Fatal("expected the surveyed accounts to be reported")
	}
	for _, a := range accounts {
		if claimed := sshtestObserved(a); len(claimed) == 0 {
			t.Errorf("account %s claimed nothing after successfully reading an empty file", a.Id())
		}
	}
}
