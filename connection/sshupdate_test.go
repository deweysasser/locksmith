package connection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deweysasser/locksmith/data"
)

// sshtestUpdateFixture stands up a sandbox holding an authorized_keys file with
// two keys in it, and returns the connection, the key library, the key that
// tests act on, the bystander key line that must survive, and the path of the
// file to assert against.
//
// The acted-on key is the shared `authorizedKey` constant, whose base64 blob
// contains a '/'.  That matters: '/' terminates a sed '/regex/' address, so a
// naive delete command is a syntax error rather than a deletion.
type sshtestUpdateFixture struct {
	conn      *SSHHostConnection
	lib       sshtestFetcher
	key       *data.SSHKey
	otherLine string
	path      string
}

func sshtestNewUpdateFixture(t *testing.T, sudo bool) sshtestUpdateFixture {
	t.Helper()
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	key := sshtestNewKey(t, authorizedKey+" alice@example.com")
	otherLine := sshtestGenerateKeyLine(t, "bob@example.com")

	path := sshtestWriteAuthorizedKeys(t, home,
		authorizedKey+" alice@example.com",
		otherLine,
	)

	return sshtestUpdateFixture{
		conn:      &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: sudo},
		lib:       sshtestFetcher{key.Id(): key},
		key:       key,
		otherLine: otherLine,
		path:      path,
	}
}

func (f sshtestUpdateFixture) binding() data.KeyBindingImpl {
	return data.KeyBindingImpl{KeyID: f.key.Id(), Location: data.AUTHORIZED_KEYS}
}

// keyBlob is the base64 body of the acted-on key, which is what actually has to
// appear in or vanish from authorized_keys.
func (f sshtestUpdateFixture) keyBlob(t *testing.T) string {
	t.Helper()
	b := f.binding()
	line, err := b.GetSshLine(f.lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		t.Fatalf("ssh line %q has no key material", line)
	}
	return fields[1]
}

// Removing a key must take out exactly that key's line and leave the rest of
// the file alone.  Against the unfixed delKey this fails: the format arguments
// were swapped, so the command deleted lines matching the *username* from a
// path built out of the key blob.
func TestUpdateRemovesKey(t *testing.T) {
	f := sshtestNewUpdateFixture(t, true)
	acct := data.NewSSHAccount("alice", "alice@host.example.com", f.conn.Id(), nil)

	if err := f.conn.Update(acct, nil, []data.KeyBindingImpl{f.binding()}, f.lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := sshtestReadFile(t, f.path)

	if strings.Contains(got, f.keyBlob(t)) {
		t.Errorf("the removed key is still in authorized_keys:\n%s", got)
	}
	if !strings.Contains(got, f.otherLine) {
		t.Errorf("the other account's key was destroyed; authorized_keys is now:\n%s", got)
	}
}

// The same, over the non-sudo code path, where both the prefix and the home
// directory component are empty.
func TestUpdateRemovesKeyWithoutSudo(t *testing.T) {
	f := sshtestNewUpdateFixture(t, false)
	acct := data.NewSSHAccount("alice", "alice@host.example.com", f.conn.Id(), nil)

	if err := f.conn.Update(acct, nil, []data.KeyBindingImpl{f.binding()}, f.lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := sshtestReadFile(t, f.path)

	if strings.Contains(got, f.keyBlob(t)) {
		t.Errorf("the removed key is still in authorized_keys:\n%s", got)
	}
	if !strings.Contains(got, f.otherLine) {
		t.Errorf("the other account's key was destroyed; authorized_keys is now:\n%s", got)
	}
}

func TestUpdateAddsKey(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	existing := sshtestGenerateKeyLine(t, "bob@example.com")
	path := sshtestWriteAuthorizedKeys(t, home, existing)

	key := sshtestNewKey(t, authorizedKey+" alice@example.com")
	lib := sshtestFetcher{key.Id(): key}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	if err := c.Update(acct, []data.KeyBindingImpl{binding}, nil, lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := sshtestReadFile(t, path)

	wantLine, err := binding.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}
	if !strings.Contains(got, wantLine) {
		t.Errorf("authorized_keys does not hold the added key line %q:\n%s", wantLine, got)
	}
	if !strings.Contains(got, existing) {
		t.Errorf("the pre-existing key was lost; authorized_keys is now:\n%s", got)
	}
	if strings.Count(got, "\n") != 2 {
		t.Errorf("expected exactly two key lines, got:\n%s", got)
	}
}

// Rotation is an add followed by a remove in a single Update, and the add has
// to happen first so a failure never leaves an account locked out.
func TestUpdateRotatesKey(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	oldKey := sshtestNewKey(t, authorizedKey+" alice@example.com")
	newLine := sshtestGenerateKeyLine(t, "alice-new@example.com")
	newKey := sshtestNewKey(t, newLine)

	path := sshtestWriteAuthorizedKeys(t, home, authorizedKey+" alice@example.com")

	lib := sshtestFetcher{oldKey.Id(): oldKey, newKey.Id(): newKey}
	add := data.KeyBindingImpl{KeyID: newKey.Id(), Location: data.AUTHORIZED_KEYS}
	remove := data.KeyBindingImpl{KeyID: oldKey.Id(), Location: data.AUTHORIZED_KEYS}

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	if err := c.Update(acct, []data.KeyBindingImpl{add}, []data.KeyBindingImpl{remove}, lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := sshtestReadFile(t, path)

	addedLine, err := add.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}
	if !strings.Contains(got, addedLine) {
		t.Errorf("the new key is missing from authorized_keys:\n%s", got)
	}
	if strings.Contains(got, strings.Fields(authorizedKey)[1]) {
		t.Errorf("the old key is still in authorized_keys:\n%s", got)
	}
}

// Nothing to do is not an error, and must not disturb the file.
func TestUpdateWithNoBindings(t *testing.T) {
	f := sshtestNewUpdateFixture(t, true)
	acct := data.NewSSHAccount("alice", "alice@host.example.com", f.conn.Id(), nil)

	before := sshtestReadFile(t, f.path)

	if err := f.conn.Update(acct, nil, nil, f.lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if after := sshtestReadFile(t, f.path); after != before {
		t.Errorf("authorized_keys changed with nothing to do:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestUpdateRejectsNonSSHAccount(t *testing.T) {
	sshtestQuiet(t)

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com"}
	acct := data.NewAWSAccount("123456789012", c.Id(), nil)

	err := c.Update(acct, nil, nil, sshtestFetcher{})
	if err == nil {
		t.Fatal("Update should reject an account that is not an SSH account")
	}
	if !strings.Contains(err.Error(), "SSHAccount") {
		t.Errorf("error %q should say the account is not an SSH account", err)
	}
}

// A binding naming a key the library does not hold is an error, not a silent
// no-op, on both the add and the remove side.
func TestUpdateUnknownKey(t *testing.T) {
	f := sshtestNewUpdateFixture(t, true)
	acct := data.NewSSHAccount("alice", "alice@host.example.com", f.conn.Id(), nil)
	missing := data.KeyBindingImpl{KeyID: "no-such-key", Location: data.AUTHORIZED_KEYS}

	if err := f.conn.Update(acct, []data.KeyBindingImpl{missing}, nil, f.lib); err == nil {
		t.Error("adding an unknown key should fail")
	}
	if err := f.conn.Update(acct, nil, []data.KeyBindingImpl{missing}, f.lib); err == nil {
		t.Error("removing an unknown key should fail")
	}
}

// If the add fails there must be no removal, or a botched rotation would lock
// the account out.
func TestUpdateAbortsRemovalWhenAddFails(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "tee() { return 1; }")

	// The key being added must NOT already be in the file: addKey now checks
	// before appending, so a key that is already present is a no-op and there
	// would be no add left to fail.  Removal targets a different key that is
	// present, so a successful run would visibly change the file.
	key := sshtestNewKey(t, authorizedKey+" alice@example.com")
	doomed := sshtestNewKey(t, sshtestGenerateKeyLine(t, "bob@example.com"))
	path := sshtestWriteAuthorizedKeys(t, home, doomed.PublicKeyString())

	lib := sshtestFetcher{key.Id(): key, doomed.Id(): doomed}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}
	removal := data.KeyBindingImpl{KeyID: doomed.Id(), Location: data.AUTHORIZED_KEYS}

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	before := sshtestReadFile(t, path)

	err := c.Update(acct, []data.KeyBindingImpl{binding}, []data.KeyBindingImpl{removal}, lib)
	if err == nil {
		t.Fatal("Update should report the failed add")
	}

	if after := sshtestReadFile(t, path); after != before {
		t.Errorf("keys were removed even though the add failed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// NewSshCmd failing has to surface as an Update error rather than a panic.
func TestUpdateFailsWhenSshCannotStart(t *testing.T) {
	sshtestQuiet(t)
	t.Setenv("LOCKSMITH_SSH", "/nonexistent/definitely-not-an-ssh-binary")

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com"}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	// Non-empty on purpose: Update skips connecting at all when there is
	// nothing to do, so an empty change would succeed here for the right
	// reason and stop testing anything.
	add := []data.KeyBindingImpl{{KeyID: "somekey", Location: data.AUTHORIZED_KEYS}}

	if err := c.Update(acct, add, nil, sshtestFetcher{}); err == nil {
		t.Error("Update should fail when the ssh command cannot be started")
	}
}

// The commonest plan is a pure revocation, and the second commonest a pure
// addition. Each used to pay a full SSH handshake for the half of Update that
// had nothing to do.
func TestUpdateDoesNotConnectWhenThereIsNothingToDo(t *testing.T) {
	sshtestQuiet(t)
	t.Setenv("LOCKSMITH_SSH", "/nonexistent/definitely-not-an-ssh-binary")

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com"}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	if err := c.Update(acct, nil, nil, sshtestFetcher{}); err != nil {
		t.Errorf("Update connected for an empty change: %v", err)
	}
}

// A key comment is free text read out of an authorized_keys file on a surveyed
// host, or out of a third-party API. It used to be pasted between single quotes
// into a command that apply then ran on a remote host, often under sudo, so a
// comment carrying a quote and a semicolon meant arbitrary command execution
// wherever that key was later placed. This drives the real Update path with
// such a comment and asserts nothing but the intended write happened.
func TestUpdateNeutralisesAMaliciousKeyComment(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	marker := filepath.Join(t.TempDir(), "pwned")
	payload := `x'; touch ` + marker + `; echo '`

	path := sshtestWriteAuthorizedKeys(t, home, sshtestGenerateKeyLine(t, "bob@example.com"))

	key := sshtestNewKey(t, authorizedKey+" "+payload)
	lib := sshtestFetcher{key.Id(): key}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	if err := c.Update(acct, []data.KeyBindingImpl{binding}, nil, lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("the key comment escaped quoting and executed: %s exists", marker)
	}

	// The key should still have been added, comment and all -- escaping must
	// neutralise the payload, not silently drop the key.
	got := sshtestReadFile(t, path)
	if !strings.Contains(got, payload) {
		t.Errorf("the key line was not written verbatim; authorized_keys is:\n%s", got)
	}
}

// The same payload by way of a removal, which builds a different command.
func TestDeleteNeutralisesAMaliciousKeyComment(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	marker := filepath.Join(t.TempDir(), "pwned")
	payload := `x'; touch ` + marker + `; echo '`

	key := sshtestNewKey(t, authorizedKey+" "+payload)
	lib := sshtestFetcher{key.Id(): key}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}

	line, err := binding.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}
	path := sshtestWriteAuthorizedKeys(t, home, line, sshtestGenerateKeyLine(t, "bob@example.com"))

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	if err := c.Update(acct, nil, []data.KeyBindingImpl{binding}, lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("the key comment escaped quoting and executed: %s exists", marker)
	}
	if got := sshtestReadFile(t, path); strings.Contains(got, payload) {
		t.Errorf("the key was not removed; authorized_keys is:\n%s", got)
	}
}

// A username reaches the command after a "~", where it cannot be quoted, so it
// is vetted instead. Update must refuse rather than build the command.
func TestUpdateRefusesAMaliciousUsername(t *testing.T) {
	sshtestQuiet(t)
	sshtestUseSandbox(t, "")

	key := sshtestNewKey(t, authorizedKey+" alice@example.com")
	lib := sshtestFetcher{key.Id(): key}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice; touch /tmp/pwned", "alice@host.example.com", c.Id(), nil)

	if err := c.Update(acct, []data.KeyBindingImpl{binding}, nil, lib); err == nil {
		t.Error("Update accepted a username containing shell metacharacters")
	}
}

// SSHHostConnection is the one Changer in the tree; apply degrades to a warning
// for every host if it ever stops satisfying the interface.
var _ Changer = (*SSHHostConnection)(nil)

// fetch unions bindings and never drops one, so an already-applied rotation is
// re-planned on the next run. A bare `tee -a` would append another copy of the
// key every cycle, growing authorized_keys without bound.
func TestUpdateDoesNotDuplicateAnAlreadyPresentKey(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	key := sshtestNewKey(t, authorizedKey+" alice@example.com")
	lib := sshtestFetcher{key.Id(): key}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}

	line, err := binding.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}
	path := sshtestWriteAuthorizedKeys(t, home, sshtestGenerateKeyLine(t, "bob@example.com"))

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	for i := 0; i < 3; i++ {
		if err := c.Update(acct, []data.KeyBindingImpl{binding}, nil, lib); err != nil {
			t.Fatalf("Update %d: %v", i+1, err)
		}
	}

	got := sshtestReadFile(t, path)
	if n := strings.Count(got, line); n != 1 {
		t.Errorf("the key appears %d times after three applies, want 1:\n%s", n, got)
	}
}

// shellQuote makes the shell see one word, but `echo` is not a transparent
// sink: in dash, ash and zsh its builtin expands \n *after* quote removal. A
// key comment is free text an unprivileged user wrote into their own
// authorized_keys on a surveyed host, so a comment carrying a literal
// backslash-n would append a second line -- injecting a key of the attacker's
// choosing onto whatever account apply was writing, under sudo.
func TestUpdateDoesNotLetAKeyCommentInjectASecondLine(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	payload := `x\nssh-rsa AAAAATTACKER attacker@example.com`

	path := sshtestWriteAuthorizedKeys(t, home, sshtestGenerateKeyLine(t, "bob@example.com"))

	key := sshtestNewKey(t, authorizedKey+" "+payload)
	lib := sshtestFetcher{key.Id(): key}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	if err := c.Update(acct, []data.KeyBindingImpl{binding}, nil, lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := sshtestReadFile(t, path)

	// Two lines: bob's, and the one key we asked for. Three means the payload
	// split into an extra authorized_keys entry.
	if n := strings.Count(strings.TrimRight(got, "\n"), "\n") + 1; n != 2 {
		t.Errorf("authorized_keys has %d lines, want 2 -- the comment injected an entry:\n%s", n, got)
	}
	// The payload must appear *within* the legitimate key's comment, not as a
	// line of its own.  (Asserting on a trailing newline would be wrong here:
	// printf supplies one, so the last legitimate line ends that way too.)
	for _, l := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if strings.HasPrefix(l, "ssh-rsa AAAAATTACKER") {
			t.Errorf("the attacker's key landed on its own line:\n%s", got)
		}
	}
}

// The duplicate guard must key on the key's identity, not the rendered line.
// Comments merge across sightings and are sorted, so the same key renders
// differently between runs; a whole-line match fails open and re-appends.
func TestUpdateDoesNotReappendWhenTheCommentChanges(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	key := sshtestNewKey(t, authorizedKey+" first@example.com")
	lib := sshtestFetcher{key.Id(): key}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}

	line, err := binding.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}
	path := sshtestWriteAuthorizedKeys(t, home, line)

	// A later sighting adds a comment that sorts earlier, so the rendered line
	// changes while the key does not.
	key.Comments.Add("aaa@example.com")

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	if err := c.Update(acct, []data.KeyBindingImpl{binding}, nil, lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	blob, err := binding.PublicKeyBlob(lib)
	if err != nil {
		t.Fatalf("PublicKeyBlob: %v", err)
	}
	if n := strings.Count(sshtestReadFile(t, path), blob); n != 1 {
		t.Errorf("the key appears %d times after the comment changed, want 1", n)
	}
}

// The escalation this guards against, end to end: a key confined to a forced
// command is rotated, and its replacement must land under the same
// restrictions. Dropping them turns a read-only rsync into an interactive
// shell on every host the key was bound to, and nothing in `plan` shows it.
func TestUpdateWritesTheReplacementUnderTheOriginalRestrictions(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	const opts = `command="/usr/bin/rrsync -ro /srv",restrict`

	oldKey := sshtestNewKey(t, authorizedKey+" backup@example.com")
	newLine := sshtestGenerateKeyLine(t, "backup-2026@example.com")
	newKey := sshtestNewKey(t, newLine)

	path := sshtestWriteAuthorizedKeys(t, home, opts+" "+authorizedKey+" backup@example.com")

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com"}
	acct := data.NewSSHAccount("backup", "backup@host.example.com", c.Id(), nil)
	lib := sshtestFetcher{oldKey.Id(): oldKey, newKey.Id(): newKey}

	// Exactly what plan's replace path produces: the same binding, carrying
	// the same options, with the key swapped.
	bound := data.KeyBindingImpl{KeyID: oldKey.Id(), Location: data.AUTHORIZED_KEYS, Options: opts}
	replacement := bound
	replacement.KeyID = newKey.Id()

	if err := c.Update(acct, []data.KeyBindingImpl{replacement}, []data.KeyBindingImpl{bound}, lib); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := sshtestReadFile(t, path)

	newBlob := strings.Fields(newLine)[1]
	var found string
	for _, l := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if strings.Contains(l, newBlob) {
			found = l
		}
	}
	if found == "" {
		t.Fatalf("the replacement key was never written:\n%s", got)
	}
	if !strings.HasPrefix(found, opts+" ") {
		t.Errorf("the replacement lost its restrictions.\nwant prefix: %s\ngot line:    %s", opts, found)
	}

	// ...and the old, restricted key is gone.
	if strings.Contains(got, strings.Fields(authorizedKey)[1]) {
		t.Errorf("the rotated-out key is still authorized:\n%s", got)
	}
}

// Options are read off the line they were found on and recorded against the
// binding, not the key: the same key is routinely unrestricted on one host and
// confined on another.
func TestFetchRecordsAuthorizedKeysOptionsOnTheBinding(t *testing.T) {
	sshtestQuiet(t)
	home := sshtestUseSandbox(t, "")

	const opts = `from="10.0.0.0/8,!10.1.2.3",no-pty,no-port-forwarding`
	sshtestWriteAuthorizedKeys(t, home,
		opts+" "+authorizedKey+" restricted@example.com",
		sshtestGenerateKeyLine(t, "unrestricted@example.com"),
	)

	cmd, err := NewSshCmd("host.example.com")
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
		t.Fatalf("got %d keys, want 2", len(keys))
	}

	var withOpts, withoutOpts int
	for _, k := range keys {
		switch k.Options {
		case opts:
			withOpts++
		case "":
			withoutOpts++
		default:
			t.Errorf("unexpected options %q", k.Options)
		}
	}
	if withOpts != 1 || withoutOpts != 1 {
		t.Errorf("got %d restricted and %d unrestricted, want 1 of each", withOpts, withoutOpts)
	}
}
