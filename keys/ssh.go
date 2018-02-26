package keys

import (
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
	"encoding/base64"
)

type SSHPublicKey struct {
	Type, PublicKeyString, PrivateKeyString string
	Comments, Options []string
}

func (key *SSHPublicKey) Json() ([]byte, error) {
	return json.Marshal(key)
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

func parseSshPrivateKey(content string) Key {
	signer, err := ssh.ParsePrivateKey([]byte(content))
	check(err)
	pub := signer.PublicKey()
	return &SSHPublicKey{pub.Type(), base64.StdEncoding.EncodeToString(pub.Marshal()), "", []string{}, []string{}}
}

func parseSshPublicKey(content string) Key {
	pub, comment, options, _, err := ssh.ParseAuthorizedKey([]byte(content))
	check(err)
	return &SSHPublicKey{pub.Type(), base64.StdEncoding.EncodeToString(pub.Marshal()), "", []string{comment}, options}
}

func SSHLoadJson(s []byte) Key {
	var key SSHPublicKey
	json.Unmarshal(s, &key)

	return &key
}
