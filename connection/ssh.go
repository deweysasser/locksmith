package connection

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/oldlib"
	"fmt"
	"strings"
	"os/exec"
)


type SSHHostConnection struct {
	connection string
}

func (c *SSHHostConnection) 	Fetch(alib *oldlib.Accountlib, klib *oldlib.KeyLib) {
	fmt.Printf("Retrieving from %s\n", c.connection)

	a := alib.EnsureAccount(c.connection)
	keys := c.RetrieveKeys()
	a.SetKeys(keys)
	for _, k:= range(keys) {
		klib.Ingest(k)
	}
}


func (remote *SSHHostConnection) RetrieveKeys() []data.Key {
	cmd := exec.Command("ssh",
		remote.connection,
		"cat",
		"~/.ssh/authorized_keys")

	out, err := cmd.Output();
	
	if err != nil {
		fmt.Printf("Failed to connect to %s: %s\n", remote.connection, err)
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
