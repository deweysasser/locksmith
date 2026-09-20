package connection

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"golang.org/x/crypto/ssh"
)

// The helpers in this file stand up a fake `ssh` so the SSH connection code can
// be exercised without a network.  get_ssh_command() honours $LOCKSMITH_SSH,
// and SshCmd speaks a protocol a plain shell already satisfies: it writes
// "<cmd>\n" followed by "echo <boundary>: $?\n" and reads until the boundary.

// sshtestRequireShell skips the calling test where /bin/sh is not available.
func sshtestRequireShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake ssh harness is a POSIX shell script")
	}
}

// sshtestScript returns the absolute path of a script in testdata/, making sure
// it is executable (git does not always preserve the mode bit).
func sshtestScript(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("resolving %s: %v", name, err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	return path
}

// sshtestUsePlainShell points $LOCKSMITH_SSH at a fake ssh that is simply a
// local shell.  Good enough for anything that only cares about command output.
func sshtestUsePlainShell(t *testing.T) {
	t.Helper()
	sshtestRequireShell(t)
	t.Setenv("LOCKSMITH_SSH", sshtestScript(t, "fakessh"))
}

// sshtestUseSandbox points $LOCKSMITH_SSH at a fake ssh that really runs the
// commands it is given, but with `sudo` stripped and every `~user/` path
// redirected into the returned sandbox directory.  That makes `tee -a` and
// `sed -i` operate on real files, so tests can assert on the result.
//
// prologue, when non-empty, is shell sourced by the fake before it starts
// reading commands; use it to stub things like `getent`.
func sshtestUseSandbox(t *testing.T, prologue string) string {
	t.Helper()
	sshtestRequireShell(t)

	home := t.TempDir()
	t.Setenv("LOCKSMITH_SSH", sshtestScript(t, "fakessh-sandbox"))
	t.Setenv("HOME", home)

	if prologue != "" {
		path := filepath.Join(t.TempDir(), "prologue.sh")
		if err := os.WriteFile(path, []byte(prologue), 0o644); err != nil {
			t.Fatalf("writing prologue: %v", err)
		}
		t.Setenv("FAKESSH_PROLOGUE", path)
	} else {
		t.Setenv("FAKESSH_PROLOGUE", "")
	}

	return home
}

// sshtestQuiet silences everything but output.Error, which ignores the level.
func sshtestQuiet(t *testing.T) {
	t.Helper()
	saved := output.Level
	output.Level = output.SilentLevel
	t.Cleanup(func() { output.Level = saved })
}

// sshtestFetcher is a stand-in for lib.KeyLibrary, which connection/ cannot
// import without a cycle.
type sshtestFetcher map[data.ID]data.Key

func (f sshtestFetcher) Fetch(id data.ID) (data.Key, error) {
	if k, ok := f[id]; ok {
		return k, nil
	}
	return nil, errors.New("no such key " + string(id))
}

// sshtestNewKey parses an authorized_keys line into the SSHKey the connection
// code would have stored.
func sshtestNewKey(t *testing.T, line string) *data.SSHKey {
	t.Helper()
	k := data.NewKey(line, time.Now())
	if k == nil {
		t.Fatalf("failed to parse key line %q", line)
	}
	sshKey, ok := k.(*data.SSHKey)
	if !ok {
		t.Fatalf("parsed key is %T, want *data.SSHKey", k)
	}
	return sshKey
}

// sshtestGenerateKeyLine makes a fresh authorized_keys line, for use as a
// bystander key that must survive whatever the test does to its neighbours.
func sshtestGenerateKeyLine(t *testing.T, comment string) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("converting key: %v", err)
	}
	return fmt.Sprintf("%s %s %s",
		sshPub.Type(),
		base64.StdEncoding.EncodeToString(sshPub.Marshal()),
		comment)
}

// sshtestWriteAuthorizedKeys drops an authorized_keys file into dir/.ssh.
func sshtestWriteAuthorizedKeys(t *testing.T, dir string, lines ...string) string {
	t.Helper()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("creating %s: %v", sshDir, err)
	}
	path := filepath.Join(sshDir, "authorized_keys")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func sshtestReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
