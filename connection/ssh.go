package connection

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"os/exec"
	"strings"
	"time"
	"fmt"
	"math/rand"
	"github.com/remeh/sizedwaitgroup"
)

type SSHHostConnection struct {
	Type       string
	Connection string
	Sudo 		bool `json:",omitempty"`
}

func (c *SSHHostConnection) Id() data.ID {
	return data.IdFromString(c.Connection)
}

func (c *SSHHostConnection) String() string {
	return "ssh://" + c.Connection
}

func (c *SSHHostConnection) Fetch() (keys <- chan data.Key, accounts <- chan data.Account) {
	if c.Sudo {
		return c.fetchSudo()
	} else {
		return c.fetchNonSudo()
	}
}

type remoteAccount struct {
	User, Home string
}

func (c *SSHHostConnection) retreiveSystemUsers() <- chan remoteAccount {
	users := make(chan remoteAccount)

	go func() {
		cmd := exec.Command("ssh",
			c.Connection,
			"getent",
			"passwd")

		out, err := cmd.Output()

		if err != nil {
			output.Errorf("Failed to connect to %s: %s\n", c.Connection, err)
		}

		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			if parts := strings.Split(l, ":"); len(parts)>5 {
				output.Debug(fmt.Sprintf("Remote %s found user %s", c.Connection, parts[0]))
				users <- remoteAccount{parts[0], parts[5]}
			}
		}

		close(users)
	}()

	return users
}

func buildAccountName(account remoteAccount, connection string) string {
	c := connection
	if i := strings.Index(connection, "@"); i > -1 {
		c = connection[(i+1):]
	}
	return fmt.Sprintf("%s@%s", account.User, c)
}

func (c *SSHHostConnection) fetchSudo() (keys <- chan data.Key, accounts <- chan data.Account) {
	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	go func() {
		output.Debugf("Retrieving from %s\n", c.Connection)

		accounts := c.retreiveSystemUsers()


		wg := sizedwaitgroup.New(10)
		for account := range accounts {
			//wg.Add(1)
			wg.Add()
			go func(account remoteAccount) {
				defer wg.Done()
				keys := c.retrieveKeysFor(account, "sudo")
				accountName := buildAccountName(account, c.Connection)
				acct := data.NewSSHAccount(accountName, c.Id(), nil)
				output.Debug("Discovered", len(keys), "keys for account", accountName)
				for _, k := range keys {
					acct.AddBinding(k)
					cKeys <- k
				}
				if len(acct.Bindings()) > 0 {
					cAccounts <- acct
				}
			}(account)
		}

		go func() {
			output.Debug("Waiting for completion")
			wg.Wait()
			output.Debug("Closing sudo fetch channels")
			close(cKeys)
			close(cAccounts)
		}()
	}()

	return cKeys, cAccounts
}

func (c *SSHHostConnection) fetchNonSudo() (keys <- chan data.Key, accounts <- chan data.Account) {
	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	go func() {
		output.Debugf("Retrieving from %s\n", c.Connection)

		acct := data.NewSSHAccount(c.Connection, c.Id(), nil)

		keys := c.RetrieveKeys()
		//a.SetKeys(keys)
		for _, k := range keys {
			acct.AddBinding(k)
			cKeys <- k
		}

		if len(acct.Bindings()) > 0 {
			cAccounts <- acct
		}
		close(cKeys)
		close(cAccounts)
	}()
	return cKeys, cAccounts
}

func (c *SSHHostConnection) retrieveKeysFor(account remoteAccount, prefix string) []data.Key {
	return c.retrieveKeysFrom(fmt.Sprintf("%s/.ssh/authorized_keys", account.Home), prefix)
}

func (remote *SSHHostConnection) RetrieveKeys() []data.Key {
	return remote.retrieveKeysFrom(".ssh/authorized_keys", "")
}

func (remote *SSHHostConnection) retrieveKeysFrom(file string, prefix string) []data.Key {
	remoteCmd := fmt.Sprintf("%s test -f %s && %s cat %s || exit 0", prefix, file, prefix, file)
	output.Debug("Running ssh", remote.Connection, remoteCmd)

	delay := time.Duration(rand.Int31()%500)
	time.Sleep(delay * time.Millisecond)
	cmd := exec.Command("ssh", remote.Connection, remoteCmd)

	if out, err := cmd.Output(); err == nil {

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
		if ee, ok := err.(*exec.ExitError); ok {
			output.Errorf("Failed to retrieve with ssh %s '%s': %s\n", remote.Connection, remoteCmd, err)
			output.Error(string(ee.Stderr))
		} else {
			output.Errorf("Failed to retrieve with ssh %s '%s': %s\n", remote.Connection, remoteCmd, err)
		}
	}

	return []data.Key{}
}

func parseAuthorizedKey(line string, t time.Time) data.Key {
	key := data.NewKey(line, t)
	if key != nil {
		return key
	}
	return nil
}
