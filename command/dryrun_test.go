package command

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
)

// dryRunFixture is a repository with one pending change against a real SSH
// connection, which is the only Changer locksmith has.
type dryRunFixture struct {
	ml   lib.MainLibrary
	conn *connection.SSHHostConnection
	acct data.Account
	key  *data.SSHKey
}

func newDryRunFixture(t *testing.T) *dryRunFixture {
	t.Helper()

	ml := lib.MainLibrary{Path: t.TempDir()}

	conn := &connection.SSHHostConnection{Type: "SSHHostConnection", Connection: "host.example.com"}
	if err := ml.Connections().Store(conn); err != nil {
		t.Fatalf("storing connection: %v", err)
	}

	key := data.NewKey("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL2n+ycV8bJgb8k6ScLmYrWZ8KrB0lqfHGx3Hs6kGvXt alice@example.com",
		time.Time{}).(*data.SSHKey)
	if err := ml.Keys().Store(key); err != nil {
		t.Fatalf("storing key: %v", err)
	}

	acct := data.NewSSHAccount("alice", "host.example.com", conn.Id(), nil)
	if err := ml.Accounts().Store(acct); err != nil {
		t.Fatalf("storing account: %v", err)
	}

	if err := ml.Changes().Store(data.Change{
		Type:    "Change",
		Account: acct.Id(),
		Add:     []data.KeyBindingImpl{{KeyID: key.Id(), Location: data.AUTHORIZED_KEYS}},
	}); err != nil {
		t.Fatalf("storing change: %v", err)
	}

	return &dryRunFixture{ml: ml, conn: conn, acct: acct, key: key}
}

func (f *dryRunFixture) pendingChanges(t *testing.T) int {
	t.Helper()
	n := 0
	for range f.ml.Changes().List() {
		n++
	}
	return n
}

// A dry run must leave the repository exactly as it found it. Deleting the
// change would be the worst outcome: the work would look done while nothing had
// happened on any host.
func TestDryRunApplyLeavesThePendingChangeAlone(t *testing.T) {
	silence(t)
	f := newDryRunFixture(t)

	// It must not need a reachable host either.
	t.Setenv("LOCKSMITH_SSH", "/nonexistent/definitely-not-an-ssh-binary")

	if before := f.pendingChanges(t); before != 1 {
		t.Fatalf("fixture has %d changes, want 1", before)
	}

	if err := dryRunApply(&f.ml, AcceptAll); err != nil {
		t.Fatalf("dryRunApply: %v", err)
	}

	if after := f.pendingChanges(t); after != 1 {
		t.Errorf("the pending change count went to %d; a dry run must change nothing", after)
	}
}

// ...and it must actually print the commands, or it is not a dry run.
func TestDryRunApplyPrintsTheRealCommands(t *testing.T) {
	f := newDryRunFixture(t)
	t.Setenv("LOCKSMITH_SSH", "/nonexistent/definitely-not-an-ssh-binary")

	got := captureOutput(t, func() {
		if err := dryRunApply(&f.ml, AcceptAll); err != nil {
			t.Fatalf("dryRunApply: %v", err)
		}
	})

	// The same command the connection would run, not a paraphrase of it.
	want, err := f.conn.Preview(f.acct,
		[]data.KeyBindingImpl{{KeyID: f.key.Id(), Location: data.AUTHORIZED_KEYS}}, nil, f.ml.Keys())
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(want) != 1 {
		t.Fatalf("expected one command to preview, got %v", want)
	}

	if !strings.Contains(got, want[0]) {
		t.Errorf("the printed command is not the one that would run.\nwant to find:\n  %s\ngot:\n%s", want[0], got)
	}
	if !strings.Contains(got, "nothing was executed") {
		t.Errorf("the dry run did not say it executed nothing:\n%s", got)
	}
}

// A connection that cannot change keys at all must be reported as skipped,
// rather than silently producing no commands -- which would read as "this
// account is already in the desired state".
func TestDryRunApplyReportsAReadOnlyConnection(t *testing.T) {
	f := newDryRunFixture(t)

	ro := &connection.GitHubConnection{Type: "GitHubConnection", User: "someone"}
	if err := f.ml.Connections().Store(ro); err != nil {
		t.Fatalf("storing connection: %v", err)
	}
	acct := data.NewSSHAccount("someone", "someone@github.com", ro.Id(), nil)
	if err := f.ml.Accounts().Store(acct); err != nil {
		t.Fatalf("storing account: %v", err)
	}
	if err := f.ml.Changes().Store(data.Change{
		Type: "Change", Account: acct.Id(),
		Remove: []data.KeyBindingImpl{{KeyID: f.key.Id(), Location: data.AUTHORIZED_KEYS}},
	}); err != nil {
		t.Fatalf("storing change: %v", err)
	}

	got := captureOutput(t, func() {
		if err := dryRunApply(&f.ml, AcceptAll); err != nil {
			t.Fatalf("dryRunApply: %v", err)
		}
	})

	if !strings.Contains(got, "cannot change keys") {
		t.Errorf("a read-only connection was not reported as skipped:\n%s", got)
	}
}

// captureOutput collects what the output package prints, which goes straight to
// os.Stdout via fmt.Println.
func captureOutput(t *testing.T, f func()) string {
	t.Helper()

	saved, savedLevel := os.Stdout, output.Level
	output.Level = output.NormalLevel

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()

	func() {
		defer func() {
			os.Stdout = saved
			output.Level = savedLevel
			w.Close()
		}()
		f()
	}()

	return <-done
}
