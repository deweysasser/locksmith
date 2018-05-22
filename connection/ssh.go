package connection

import (
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"math/rand"
	"strings"
	"time"
	"errors"
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

func (c *SSHHostConnection) Fetch() (keys <-chan data.Key, accounts <-chan data.Account) {
	if c.Sudo {
		return c.fetchSudo()
	} else {
		return c.fetchNonSudo()
	}
}

func (c *SSHHostConnection) Update(account data.Account, addBindings []data.KeyBinding, removeBindings []data.KeyBinding, keylib data.Fetcher) error {
	if c.Sudo {
		if sAcct, ok := account.(*data.SSHAccount); ok {
			return errors.New(fmt.Sprint("Unsupported account ", sAcct))
		} else {
			return errors.New("Account is not SSHAccount")
		}
	} else {
		if cmd, err := NewSshCmd(c.Connection); err != nil {
			return err
		} else {
			for _, add := range addBindings {
				if line, err := add.GetSshLine(keylib); err != nil {
					return errors.New(fmt.Sprint("Error generating SSH line: ", err))
				} else {
					if _, err := cmd.Run(fmt.Sprintf("echo '%s' >> ~/.ssh/authorized_keys", line)); err != nil {
						return err
					}
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

func (c *SSHHostConnection) fetchSudo() (keys <-chan data.Key, accounts <-chan data.Account) {
	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	go func() {
		defer close(cKeys)
		defer close(cAccounts)

		output.Debugf("Retrieving from %s\n", c.Connection)

		if ssh, err := NewSshCmd(c.Connection); err != nil {
			output.Error(fmt.Sprintf("Failed to open SSH connection to %s: %s", c.Connection, err))
		} else {

			accounts := c.retreiveSystemUsers(ssh)

			for account := range accounts {
				accountName := buildAccountName(account, c.Connection)
				output.Debug("Retrieving keys for", accountName)
				keys := c.retrieveKeysFor(ssh, account, "sudo")
				acct := data.NewSSHAccount(account.User, accountName, c.Id(), nil)
				output.Debug("Discovered", len(keys), "keys for account", accountName)
				for _, k := range keys {
					acct.AddBinding(k)
					cKeys <- k
				}
				if len(acct.Bindings()) > 0 {
					cAccounts <- acct
				}
			}
		}
	}()

	return cKeys, cAccounts
}

func (c *SSHHostConnection) fetchNonSudo() (keys <-chan data.Key, accounts <-chan data.Account) {
	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)


	go func() {
		if ssh, err := NewSshCmd(c.Connection); err != nil {
			output.Error(fmt.Sprintf("Failed to open SSH connection to %s: %s", c.Connection, err))
		} else {
			defer ssh.Close()
			defer close(cKeys)
			defer close(cAccounts)

			if iam, err := ssh.Run("whoami"); err != nil {
				output.Error(fmt.Sprint("Failed to get username: ", err))
			} else {
				output.Debugf("Retrieving from %s\n", c.Connection)

				acct := data.NewSSHAccount(c.Connection, iam, c.Id(), nil)

				keys := c.RetrieveKeys(ssh)
				//a.SetKeys(keys)
				for _, k := range keys {
					acct.AddBinding(k)
					cKeys <- k
				}

				if len(acct.Bindings()) > 0 {
					cAccounts <- acct
				}
			}
		}
	}()
	return cKeys, cAccounts
}

func (c *SSHHostConnection) retrieveKeysFor(cmd *SshCmd, account remoteAccount, prefix string) []data.Key {
	return c.retrieveKeysFrom(cmd, fmt.Sprintf("%s/.ssh/authorized_keys", account.Home), prefix)
}

func (remote *SSHHostConnection) RetrieveKeys(cmd *SshCmd) []data.Key {
	return remote.retrieveKeysFrom(cmd,".ssh/authorized_keys", "")
}

func (remote *SSHHostConnection) retrieveKeysFrom(cmd *SshCmd,  file string, prefix string) []data.Key {
	remoteCmd := fmt.Sprintf("%s cat %s", prefix, file)

	delay := time.Duration(rand.Int31() % 500)
	time.Sleep(delay * time.Millisecond)

	if out, err := cmd.Run(remoteCmd); err == nil {
		output.Debug("Parsing Returned Keys")
		lines := strings.Split(string(out), "\n")

		keys := make([]data.Key, 0)

		for _, line := range lines {
			key := parseAuthorizedKey(line, time.Now())
			if key != nil {
				keys = append(keys, key)
			}
		}
		return keys
	} else {
		output.Debug("No keys to retrieve for", file)
	}

	output.Debug("returning keys")
	return []data.Key{}
}

func parseAuthorizedKey(line string, t time.Time) data.Key {
	key := data.NewKey(line, t)
	if key != nil {
		return key
	}
	return nil
}
