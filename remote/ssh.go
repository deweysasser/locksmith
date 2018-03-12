package remote

import (
	"github.com/deweysasser/locksmith/keys"
	"fmt"
	"strings"
	"os/exec"
)


func RetrieveKeys(server string) []keys.Key {
	cmd := exec.Command("ssh",
		server,
		"cat",
		"~/.ssh/authorized_keys")

	out, err := cmd.Output();
	
	if err != nil {
		fmt.Printf("Failed to connect to %s: %s\n", server, err)
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
