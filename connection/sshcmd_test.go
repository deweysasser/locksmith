package connection

import (
	"os"
	"strings"
	"testing"
)

func TestGetSshCommandDefault(t *testing.T) {
	t.Setenv("LOCKSMITH_SSH", "")
	os.Unsetenv("LOCKSMITH_SSH")

	if got := get_ssh_command(); got != "ssh" {
		t.Errorf("get_ssh_command() = %q, want %q", got, "ssh")
	}
}

func TestGetSshCommandFromEnvironment(t *testing.T) {
	t.Setenv("LOCKSMITH_SSH", "/usr/local/bin/my-ssh")

	if got := get_ssh_command(); got != "/usr/local/bin/my-ssh" {
		t.Errorf("get_ssh_command() = %q, want the value of $LOCKSMITH_SSH", got)
	}
}

// NewSshCmd runs "true" to swallow any login banner, so anything the remote
// prints at connect time must not leak into the first real command's output.
func TestNewSshCmdDiscardsBanner(t *testing.T) {
	sshtestQuiet(t)
	sshtestUseSandbox(t, "echo 'WARNING: authorized use only'")

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}
	defer cmd.Close()

	out, err := cmd.Run("echo hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "hello" {
		t.Errorf("Run() = %q, want %q -- the banner should have been discarded", out, "hello")
	}
}

func TestSshCmdRun(t *testing.T) {
	sshtestQuiet(t)
	sshtestUsePlainShell(t)

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}
	defer cmd.Close()

	tests := []struct {
		name    string
		command string
		want    string
		wantErr string
	}{
		{"simple output", "echo hello", "hello", ""},
		{"no output", "true", "", ""},
		{"multi-line output", "printf 'one\\ntwo\\nthree\\n'", "one\ntwo\nthree", ""},
		{"non-zero exit", "false", "", "Non-zero exit: 1"},
		{"specific exit code", "(exit 3)", "", "Non-zero exit: 3"},
		{"output then failure", "echo partial; false", "partial", "Non-zero exit: 1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := cmd.Run(tc.command)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Run(%q): unexpected error %v", tc.command, err)
				}
			} else {
				if err == nil {
					t.Fatalf("Run(%q): expected error %q, got none", tc.command, tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Errorf("Run(%q) error = %q, want %q", tc.command, err.Error(), tc.wantErr)
				}
			}

			if out != tc.want {
				t.Errorf("Run(%q) = %q, want %q", tc.command, out, tc.want)
			}
		})
	}
}

// Run keeps state on the connection, so consecutive commands have to stay in
// step with each other rather than reading one another's boundary lines.
func TestSshCmdRunIsSequential(t *testing.T) {
	sshtestQuiet(t)
	sshtestUsePlainShell(t)

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}
	defer cmd.Close()

	for _, want := range []string{"first", "second", "third"} {
		if got, err := cmd.Run("echo " + want); err != nil || got != want {
			t.Fatalf("Run(echo %s) = %q, %v; want %q, nil", want, got, err, want)
		}
	}
}

// A remote shell that exits mid-stream produces EOF rather than a boundary
// line; Run has to report that rather than block or return a bogus success.
func TestSshCmdRunAfterRemoteExit(t *testing.T) {
	sshtestQuiet(t)
	sshtestUsePlainShell(t)

	cmd, err := NewSshCmd("anyhost")
	if err != nil {
		t.Fatalf("NewSshCmd: %v", err)
	}

	if _, err := cmd.Run("exit 0"); err == nil {
		t.Error("expected an error once the remote shell has gone away")
	}
	cmd.Close()
}

func TestNewSshCmdFailsForMissingCommand(t *testing.T) {
	sshtestQuiet(t)
	t.Setenv("LOCKSMITH_SSH", "/nonexistent/definitely-not-an-ssh-binary")

	if cmd, err := NewSshCmd("anyhost"); err == nil {
		cmd.Close()
		t.Error("NewSshCmd should fail when the ssh command cannot be started")
	} else if !strings.Contains(err.Error(), "definitely-not-an-ssh-binary") {
		t.Errorf("error %q should name the command that could not be started", err)
	}
}
