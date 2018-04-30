package connection

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"os/exec"
	"strings"
	"time"
)

type SSHHostConnection struct {
	Type       string
	Connection string
}

func (c *SSHHostConnection) Id() data.ID {
	return data.IdFromString(c.Connection)
}

func (c *SSHHostConnection) String() string {
	return "ssh://" + c.Connection
}

func (c *SSHHostConnection) Fetch() (keys <- chan data.Key, accounts <- chan data.Account) {
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

		cAccounts <- acct
		close(cKeys)
		close(cAccounts)
	}()
	return cKeys, cAccounts
}

func (remote *SSHHostConnection) RetrieveKeys() []data.Key {
	cmd := exec.Command("ssh",
		remote.Connection,
		"cat",
		"~/.ssh/authorized_keys")

	out, err := cmd.Output()

	if err != nil {
		output.Errorf("Failed to connect to %s: %s\n", remote.Connection, err)
	}

	lines := strings.Split(string(out), "\n")

	keys := make([]data.Key, 0)

	for _, line := range lines {
		key := parseAuthorizedKey(line, time.Now())
		if key != nil {
			keys = append(keys, key)
		}
	}

	return keys
}

func parseAuthorizedKey(line string, t time.Time) data.Key {
	key := data.NewKey(line, t)
	if key != nil {
		return key
	}
	return nil
}
