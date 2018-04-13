package connection

import (
	"github.com/deweysasser/locksmith/keys"
	"fmt"
	"strings"
	"os/exec"
)

type SSHRemote struct {
	server string
}

func NewSSHRemote(server string) *SSHRemote{
	return &SSHRemote{server}
}

func (remote *SSHRemote) RetrieveKeys() []keys.Key {
	cmd := exec.Command("ssh",
		remote.server,
		"cat",
		"~/.ssh/authorized_keys")

	out, err := cmd.Output();
	
	if err != nil {
		fmt.Printf("Failed to connect to %s: %s\n", remote.server, err)
	}

	lines := strings.Split(string(out), "\n")

	keys := make([]keys.Key,0)


	for _, line := range lines {
		key := parseAuthorizedKey(line)
		if key != nil {
			keys = append(keys, key)
		}
	}

	return keys
}

func parseAuthorizedKey(line string) keys.Key {
	key := keys.New(line)
	if key != nil {
		return key
	}
	return nil
}
