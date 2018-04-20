package connection

import (
	"github.com/deweysasser/locksmith/data"
	"fmt"
	"strings"
	"os/exec"
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

func (c *SSHHostConnection) 	Fetch(cKeys chan data.Key, cAccounts chan data.Account) {
	defer close(cKeys)
	defer close(cAccounts)

	fmt.Printf("Retrieving from %s\n", c.Connection)

	acct := data.Account{"SSH", c.Connection, c.Id(), nil}

	keys := c.RetrieveKeys()
	//a.SetKeys(keys)
	for _, k:= range(keys) {
		acct.AddBinding(k)
		cKeys <- k
	}

	cAccounts <- acct
}


func (remote *SSHHostConnection) RetrieveKeys() []data.Key {
	cmd := exec.Command("ssh",
		remote.Connection,
		"cat",
		"~/.ssh/authorized_keys")

	out, err := cmd.Output();
	
	if err != nil {
		fmt.Printf("Failed to connect to %s: %s\n", remote.Connection, err)
	}

	lines := strings.Split(string(out), "\n")

	keys := make([]data.Key,0)


	for _, line := range lines {
		key := parseAuthorizedKey(line)
		if key != nil {
			keys = append(keys, key)
		}
	}

	return keys
}

func parseAuthorizedKey(line string) data.Key {
	key := data.New(line)
	if key != nil {
		return key
	}
	return nil
}
