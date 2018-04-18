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

func (c *SSHHostConnection) 	Fetch(cKeys chan data.Key, cAccounts chan data.Account) {
	fmt.Printf("Retrieving from %s\n", c.Connection)

	cAccounts <- data.Account{"SSH", c.Connection}

	keys := c.RetrieveKeys()
	//a.SetKeys(keys)
	for _, k:= range(keys) {
		cKeys <- k
	}
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
