package connection

import (
	"fmt"
	"regexp"
	"strings"
)

// Commands sent to a remote host are assembled as shell text and evaluated by
// that host's shell (see SshCmd.Run).  Much of what goes into them comes from
// outside the trust boundary: key comments are free text read out of
// authorized_keys files on surveyed hosts and out of third-party APIs, and
// usernames and home directories come from `getent passwd` on the remote side.
//
// None of that may be pasted into a command unescaped.  Everything in this file
// exists to make interpolation safe, and every construction site in ssh.go is
// expected to use it.

// shellQuote renders s as a single POSIX shell word.
//
// The content is wrapped in single quotes, inside which the shell treats every
// character literally.  The one character that cannot appear inside such a run
// is the single quote itself; it is emitted by closing the run, escaping a
// literal quote with a backslash, and reopening.  See the replacement below.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// remoteName matches the names we are willing to paste into a command
// unquoted, which is required for the one construct that cannot tolerate
// quoting: tilde expansion.  `~root` expands to root's home directory, while
// `~'root'` is a literal string, so a username destined for `~%s` has to be
// vetted rather than escaped.
//
// This is deliberately narrower than POSIX allows for usernames.  A name it
// rejects costs an operator an error message; a name it should have rejected
// costs them a shell.
var remoteName = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._-]*$`)

// dotsOnly matches the names remoteName would otherwise admit but must not: the
// shell does not expand "~." or "~..", so they survive as relative paths and
// retarget the write at the login user's own home, or its parent.
var dotsOnly = regexp.MustCompile(`^\.+$`)

// checkRemoteName reports whether name is safe to interpolate unquoted.  An
// empty name is allowed, because the non-sudo code path uses "" to mean "the
// account we logged in as" and builds `~/.ssh/...`.
func checkRemoteName(what, name string) error {
	if name == "" {
		return nil
	}
	if !remoteName.MatchString(name) || dotsOnly.MatchString(name) {
		return fmt.Errorf("refusing to use %s %q in a remote command: it is not a plain name", what, name)
	}
	return nil
}

// sudoPrefix is the only prefix ssh.go ever sets, but it reaches the command
// string through a parameter, so it is checked rather than assumed.
const sudoPrefix = "sudo"

// checkPrefix reports whether prefix is one locksmith itself produces.
func checkPrefix(prefix string) error {
	if prefix == "" || prefix == sudoPrefix {
		return nil
	}
	return fmt.Errorf("refusing to use command prefix %q", prefix)
}
