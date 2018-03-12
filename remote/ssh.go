package remote

import (
	"github.com/deweysasser/locksmith/keys"
	"fmt"
	"strings"
	"os/exec"
)


func RetrieveKeys(server string, kchan chan keys.Key) {
	fmt.Printf("Retrieving from %s\n", server)

	cmd := exec.Command("ssh",
		server,
		"cat",
		"~/.ssh/authorized_keys")

	out, err := cmd.Output();
	
	if err != nil {
		fmt.Printf("Failed to connect to %s: %s", server, err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		fmt.Println("Found line", line)
		parseAuthorizedKey(line, kchan)
	}

}

func parseAuthorizedKey(line string, kchan chan keys.Key) {
	fmt.Println("Parsing key")
	key := keys.New(line)
	fmt.Println("Parsed key.  queuing")
	if key != nil {
		kchan <- key
	} else {
		fmt.Println("Key was nil")
	}
}
