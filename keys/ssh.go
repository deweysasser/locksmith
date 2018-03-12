package keys

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"golang.org/x/crypto/ssh"
)

type SSHPublicKey struct {
	Type, PublicKeyString, PrivateKeyString string
	Comments, Options, Names              []string
}

func (key *SSHPublicKey) String() string {
	return fmt.Sprintf("%s %s %s", key.Type, key.Id(), strings.Join(key.Comments, ", "))
}

func (key *SSHPublicKey) Json() ([]byte, error) {
	return json.MarshalIndent(key, "", "  ")
}

func (key *SSHPublicKey) IsDeprecated() bool {
	return false
}

func (key *SSHPublicKey) Replacement() string {
	return ""
}

func (key *SSHPublicKey) PublicKey() ssh.PublicKey {
	line := fmt.Sprintf("%s %s", key.Type, key.PublicKeyString)
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	check(err)
	return pubkey
}

func (key *SSHPublicKey) Id() string {
	return ssh.FingerprintSHA256(key.PublicKey())
}

func (key *SSHPublicKey) Ids() []string {
	return []string { ssh.FingerprintSHA256(key.PublicKey())}
}

func (key *SSHPublicKey) GetNames() []string {
	return key.Names
}

func parseSshPrivateKey(content string) Key {
	signer, err := ssh.ParsePrivateKey([]byte(content))
	check(err)
	pub := signer.PublicKey()
	return &SSHPublicKey{Type: pub.Type(),
		PublicKeyString: base64.StdEncoding.EncodeToString(pub.Marshal()),
		PrivateKeyString: "",
		Comments: []string{},
		Options: []string{},
		Names: []string{}}
}

func parseSshPublicKey(content string) Key {
	pub, comment, options, _, err := ssh.ParseAuthorizedKey([]byte(content))
	check(err)
	return &SSHPublicKey{Type: pub.Type(),
		PublicKeyString: base64.StdEncoding.EncodeToString(pub.Marshal()),
		PrivateKeyString: "",
		Comments: []string{comment},
		Options: options,
		Names: []string{},
	}
}

func SSHLoadJson(s []byte) Key {
	var key SSHPublicKey
	json.Unmarshal(s, &key)

	return &key
}

