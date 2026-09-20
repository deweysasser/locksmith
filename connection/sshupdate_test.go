package connection

import (
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

	key := sshtestNewKey(t, authorizedKey+" alice@example.com")
	other := sshtestGenerateKeyLine(t, "bob@example.com")
	path := sshtestWriteAuthorizedKeys(t, home, authorizedKey+" alice@example.com", other)

	lib := sshtestFetcher{key.Id(): key}
	binding := data.KeyBindingImpl{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}

	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com", Sudo: true}
	acct := data.NewSSHAccount("alice", "alice@host.example.com", c.Id(), nil)

	before := sshtestReadFile(t, path)

	err := c.Update(acct, []data.KeyBindingImpl{binding}, []data.KeyBindingImpl{binding}, lib)
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

	if err := c.Update(acct, nil, nil, sshtestFetcher{}); err == nil {
		t.Error("Update should fail when the ssh command cannot be started")
	}
}
