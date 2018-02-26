package keys

import (
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
	"strings"
)

type SSHPublicKey struct {
	Type, Content, Comment, Constraints string
}

func (key *SSHPublicKey) Json() ([]byte, error) {
	return json.Marshal(key)
}

func (key *SSHPublicKey) PublicKey() ssh.PublicKey {
	line := fmt.Sprintf("%s %s %s", key.Type, key.Content, key.Comment)
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	check(err)
	return pubkey
}

func (key *SSHPublicKey) Id() string {
	return ssh.FingerprintSHA256(key.PublicKey())
}

func parseSshPublicKey(content string) Key {
	content = strings.Trim(content, " \t\n")
	command := ""

	if strings.HasPrefix(strings.ToLower(content), "command") {
		i := strings.Index(content, " ssh-")

		command = content[:i]
		content = strings.Trim(content[i:], " \t\n")
	}

	slice := strings.SplitN(content, " ", 3)
	if len(slice) > 2 {
		return &SSHPublicKey{slice[0], slice[1], slice[2], command}
	} else {
		return &SSHPublicKey{slice[0], slice[1], "", command}
	}
}

func SSHLoadJson(s []byte) Key {
	var key SSHPublicKey
	json.Unmarshal(s, &key)

	return &key
}
