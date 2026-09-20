package connection

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type SSHHostConnection struct {
	Type       string
	Connection string
	Sudo       bool `json:",omitempty"`
}

func (c *SSHHostConnection) Id() data.ID {
	return data.IdFromString(c.Connection)
}

func (c *SSHHostConnection) String() string {
	if c.Sudo {
		return "ssh://" + c.Connection + "?sudo=true"
	}
	return "ssh://" + c.Connection
}

func (c *SSHHostConnection) Fetch(ctx context.Context) (keys <-chan data.Key, accounts <-chan data.Account) {
	if c.Sudo {
		return c.fetchSudo(ctx)
	} else {
		return c.fetchNonSudo(ctx)
	}
}

func (c *SSHHostConnection) Update(account data.Account, addBindings []data.KeyBindingImpl, removeBindings []data.KeyBindingImpl, keylib data.Fetcher) error {
	if sAcct, ok := account.(*data.SSHAccount); ok {
		if c.Sudo {
			if err := c.addKey("sudo", sAcct.Username, addBindings, keylib); err == nil {
				return c.delKey("sudo", sAcct.Username, removeBindings, keylib)
			} else {
				return errors.New("Failed to add keys so aborting removal")
			}
		} else {
			if err := c.addKey("", "", addBindings, keylib); err == nil {
				return c.delKey("", "", removeBindings, keylib)
			} else {
				return errors.New("Failed to add keys so aborting removal")
			}
		}
	} else {
		return errors.New("Account is not SSHAccount")
	}
}

func (c *SSHHostConnection) delKey(prefix string, path string, bindings []data.KeyBindingImpl, keylib data.Fetcher) error {
	if err := checkPrefix(prefix); err != nil {
		return err
	}
	// path lands after a "~", where it cannot be quoted without defeating
	// tilde expansion, so it has to be vetted instead.
	if err := checkRemoteName("username", path); err != nil {
		return err
	}

	if cmd, err := NewSshCmd(c.Connection); err != nil {
		return err
	} else {
		// Without this the ssh child is never reaped and its three pipes stay
		// open for the life of the process; apply runs this once per account.
		defer cmd.Close()
		for _, add := range bindings {
			if key, err := keylib.Fetch(add.KeyID); err == nil {
				if sshKey, ok := key.(*data.SSHKey); ok {
					text := base64.StdEncoding.EncodeToString(sshKey.PublicKey.Key.Marshal())
					// base64 may contain '/', which would end a sed '/re/'
					// address early, so select '|' as the address delimiter
					// instead.  The rest of the base64 alphabet holds no BRE
					// metacharacters and no shell single quote, so the key
					// material is safe to interpolate as-is.
					removeLine := fmt.Sprintf("%s sed -i -e '\\|%s|d' ~%s/.ssh/authorized_keys", prefix, text, path)
					if _, err := cmd.Run(removeLine); err != nil {
						return errors.New(fmt.Sprintf("Failed to run '%s': %s", removeLine, err))
					}
				} else {
					return errors.New(fmt.Sprint("Key ", add.KeyID, " is not an SSH key"))
				}
			} else {
				return errors.New(fmt.Sprint("Error looking up key ", add.KeyID))
			}
		}
	}
	return nil
}

func (c *SSHHostConnection) addKey(prefix string, path string, addBindings []data.KeyBindingImpl, keylib data.Fetcher) error {
	if err := checkPrefix(prefix); err != nil {
		return err
	}
	if err := checkRemoteName("username", path); err != nil {
		return err
	}

	if cmd, err := NewSshCmd(c.Connection); err != nil {
		return err
	} else {
		defer cmd.Close()
		for _, add := range addBindings {
			if line, err := add.GetSshLine(keylib); err != nil {
				return errors.New(fmt.Sprint("Error generating SSH line: ", err))
			} else {
				// The line ends with the key's comment, which is free text
				// taken from a surveyed host or a third-party API -- squarely
				// outside the trust boundary.  It must be escaped, not merely
				// wrapped in quotes.
				// grep before appending: `fetch` unions bindings and never
				// drops one, so a rotation that has already been applied is
				// re-planned on the next run.  With a bare `tee -a` that
				// appends another copy of the same key every cycle, without
				// bound.
				quoted := shellQuote(line)
				addLine := fmt.Sprintf("%s grep -qxF %s ~%s/.ssh/authorized_keys 2>/dev/null || echo %s | %s tee -a ~%s/.ssh/authorized_keys",
					prefix, quoted, path, quoted, prefix, path)
				if _, err := cmd.Run(addLine); err != nil {
					return errors.New(fmt.Sprintf("Failed to run '%s': %s", addLine, err))
				}
			}
		}
	}
	return nil
}

type remoteAccount struct {
	User, Home string
}

func (c *SSHHostConnection) retreiveSystemUsers(cmd *SshCmd) <-chan remoteAccount {
	users := make(chan remoteAccount)

	if out, err := cmd.Run("getent passwd"); err != nil {
		close(users)
		return users
	} else {
		go func() {
			defer close(users)

			if err != nil {
				output.Errorf("Failed to connect to %s: %s\n", c.Connection, err)
			}

			lines := strings.Split(string(out), "\n")
			for _, l := range lines {
				if parts := strings.Split(l, ":"); len(parts) > 5 {
					output.Debug(fmt.Sprintf("Remote %s found user %s", c.Connection, parts[0]))
					users <- remoteAccount{parts[0], parts[5]}
				}
			}

		}()
		return users
	}
}

func buildAccountName(account remoteAccount, connection string) string {
	c := connection
	if i := strings.Index(connection, "@"); i > -1 {
		c = connection[(i + 1):]
	}
	return fmt.Sprintf("%s@%s", account.User, c)
}

// 5 threads seems to be the optimum between connection overhead and command serilaization on a single threaded Ubuntu VM with ~30 users
var ParallelSSHCount int = 5

func (c *SSHHostConnection) fetchSudo(ctx context.Context) (keys <-chan data.Key, accounts <-chan data.Account) {
	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	var cmd *SshCmd
	var err error

	if cmd, err = NewSshCmd(c.Connection); err != nil {
		output.Error(fmt.Sprintf("Failed to open SSH connection to %s: %s", c.Connection, err))
		close(cKeys)
		close(cAccounts)
		return cKeys, cAccounts
	} else {
		defer cmd.Close()
	}

	systemAccounts := c.retreiveSystemUsers(cmd)

	wg := sync.WaitGroup{}

	wg.Add(ParallelSSHCount)

	for i := 0; i < ParallelSSHCount; i++ {
		go func(i int) {
			defer wg.Done()

			output.Debugf("Retrieving from %s on thread %d\n", c.Connection, i)

			if ssh, err := NewSshCmd(c.Connection); err != nil {
				output.Error(fmt.Sprintf("Failed to open SSH connection to %s: %s", c.Connection, err))
			} else {
				defer ssh.Close()
				for account := range systemAccounts {
					if ctx.Err() != nil {
						return
					}
					output.Debugf("Fetching account %s on thread %d\n", account, i)
					accountName := buildAccountName(account, c.Connection)
					output.Debug("Retrieving keys for", accountName)
					keys, err := c.retrieveKeysFor(ssh, account, "sudo")
					acct := data.NewSSHAccount(account.User, accountName, c.Id(), nil)
					if err == nil {
						// Only a successful read may claim completeness.  A
						// failed one yields no keys too, and claiming on that
						// would wipe the account's recorded bindings.
						acct.MarkObserved(data.AUTHORIZED_KEYS, data.UnspecifiedLocation)
					}
					output.Debug("Discovered", len(keys), "keys for account", accountName)
					for _, k := range keys {
						acct.AddBinding(k, data.AUTHORIZED_KEYS)
						if !sendKey(ctx, cKeys, k) {
							return
						}
					}
					// Emitted even with no keys: an account whose
					// authorized_keys is now empty still has to be reported, or
					// the bindings recorded for it can never be cleared.
					if !sendAccount(ctx, cAccounts, acct) {
						return
					}
				}
			}
		}(i)
	}

	go func() {
		defer close(cKeys)
		defer close(cAccounts)
		wg.Wait()
	}()

	return cKeys, cAccounts
}

func (c *SSHHostConnection) fetchNonSudo(ctx context.Context) (keys <-chan data.Key, accounts <-chan data.Account) {
	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	go func() {
		// Closing here rather than in the success branch: leaving these open on
		// a connection failure wedges the fan-in, and `fetch` hangs forever
		// with no output and no error.
		defer close(cKeys)
		defer close(cAccounts)

		if ssh, err := NewSshCmd(c.Connection); err != nil {
			output.Error(fmt.Sprintf("Failed to open SSH connection to %s: %s", c.Connection, err))
		} else {
			defer ssh.Close()

			if iam, err := ssh.Run("whoami"); err != nil {
				output.Error(fmt.Sprint("Failed to get username: ", err))
			} else {
				output.Debugf("Retrieving from %s\n", c.Connection)

				// (username, name): `iam` is the remote whoami, c.Connection
				// is the "[user@]host" we dialled.  Swapping these produced an
				// account ID of "user@host@user".
				acct := data.NewSSHAccount(iam, c.Connection, c.Id(), nil)

				keys, err := c.RetrieveKeys(ssh)
				if err == nil {
					acct.MarkObserved(data.AUTHORIZED_KEYS, data.UnspecifiedLocation)
				}
				//a.SetKeys(keys)
				for _, k := range keys {
					acct.AddBinding(k, data.AUTHORIZED_KEYS)
					if !sendKey(ctx, cKeys, k) {
						return
					}
				}

				// Emitted even with no keys, for the same reason as the sudo
				// path: an emptied authorized_keys must be able to clear the
				// bindings recorded for the account.
				sendAccount(ctx, cAccounts, acct)
			}
		}
	}()
	return cKeys, cAccounts
}

func (c *SSHHostConnection) retrieveKeysFor(cmd *SshCmd, account remoteAccount, prefix string) ([]data.Key, error) {
	return c.retrieveKeysFrom(cmd, fmt.Sprintf("%s/.ssh/authorized_keys", account.Home), prefix)
}

func (remote *SSHHostConnection) RetrieveKeys(cmd *SshCmd) ([]data.Key, error) {
	return remote.retrieveKeysFrom(cmd, ".ssh/authorized_keys", "")
}

// retrieveKeysFrom reads one authorized_keys file.
//
// The error return is load-bearing, not decoration: "the file was empty" and
// "the read failed" both produce no keys, and the caller uses the difference to
// decide whether it may claim to have observed the account's keys completely.
// Conflating them lets a transient sudo or permission failure delete every
// binding recorded for that account.
//
// A non-zero exit is deliberately treated as "do not claim", including the case
// where the file simply does not exist.  That errs towards keeping a stale
// binding rather than dropping a real one.
func (remote *SSHHostConnection) retrieveKeysFrom(cmd *SshCmd, file string, prefix string) ([]data.Key, error) {
	if err := checkPrefix(prefix); err != nil {
		output.Error(err)
		return nil, err
	}

	// file is assembled from a home directory read out of the remote host's own
	// passwd file, and this command may run under sudo -- so a local user on a
	// surveyed host could otherwise escalate through locksmith.
	remoteCmd := fmt.Sprintf("%s cat %s", prefix, shellQuote(file))

	delay := time.Duration(rand.Int31() % 500)
	time.Sleep(delay * time.Millisecond)

	out, err := cmd.Run(remoteCmd)
	if err != nil {
		output.Verbose("Could not read", file, "on", remote.Connection, ":", err)
		return nil, err
	}

	output.Debug("Parsing Returned Keys")
	keys := make([]data.Key, 0)
	for _, line := range strings.Split(string(out), "\n") {
		if key := parseAuthorizedKey(line, time.Now()); key != nil {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func parseAuthorizedKey(line string, t time.Time) data.Key {
	key := data.NewKey(line, t)
	if key != nil {
		return key
	}
	return nil
}

// sendKey and sendAccount deliver one item unless the run has been cancelled.
//
// A bare send on an unbuffered channel blocks until a consumer takes the value.
// When a fetch is abandoned nobody is consuming, so every producer goroutine
// would block there for the life of the process -- and an SSH fetch runs
// ParallelSSHCount of them per host.  They report whether the caller should
// keep going.
func sendKey(ctx context.Context, ch chan<- data.Key, k data.Key) bool {
	select {
	case ch <- k:
		return true
	case <-ctx.Done():
		return false
	}
}

func sendAccount(ctx context.Context, ch chan<- data.Account, a data.Account) bool {
	select {
	case ch <- a:
		return true
	case <-ctx.Done():
		return false
	}
}
