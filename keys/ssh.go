package keys

import (
//	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
	"strings"
)

/** An SSH key, public and (optionally) private
*/
type SSHKey struct {
	keyImpl
	PublicKey ssh.PublicKey
	Comments []string
}

/* Use of a public key, e.g. in an authorized_keys file
*/
type SSHBinding struct {
	Id KeyID
	Comment string
	Options []string
}

func NewSshKey(pub ssh.PublicKey) *SSHKey {
	return &SSHKey{keyImpl{"SSHKey", []KeyID{getId(pub)}, []string{}, false, ""}, pub, []string{}}
}

func (key *SSHKey) String() string {
	return fmt.Sprintf("%s %s %s", key.Type, key.Id(), strings.Join(key.Comments, ", "))
}

func (key *SSHKey) KeyType() string {
	return key.PublicKey.Type()
}

func (key *SSHKey) Json() ([]byte, error) {
	return json.MarshalIndent(key, "", "  ")
}

func (key *SSHKey) IsDeprecated() bool {
	return false
}

func (key *SSHKey) Replacement() KeyID {
	return ""
}

func publicKey(keytype, pub string) ssh.PublicKey {
	line := fmt.Sprintf("%s %s", keytype, pub)
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	check(err)
	return pubkey
}

func (key *SSHKey) PublicKeyString() string {
	return string(ssh.MarshalAuthorizedKey(key.PublicKey))
}

func getId(pub ssh.PublicKey) KeyID {
	return KeyID(ssh.FingerprintSHA256(pub))
}

func (key *SSHKey) GetNames() []string {
	return key.Names
}


func parseSshPrivateKey(content string) Key {
	signer, err := ssh.ParsePrivateKey([]byte(content))
	check(err)
	pub := signer.PublicKey()
	id := getId(pub)
	return &SSHKey{
		keyImpl: keyImpl{
			Type: "SSHKey",
			Ids: []KeyID{id},
			Names: []string{},
			Deprecated: false,
			Replacement: "" },
		PublicKey: pub,
		Comments:         []string{}	}
}

func parseSshPublicKey(content string) Key {
	//	pub, comment, options, _, err := ssh.ParseAuthorizedKey([]byte(content))
	pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(content))
	check(err)
	return &SSHKey{
		keyImpl: keyImpl{
			Type: "SSHKey",
			Ids: []KeyID{getId(pub)},
			Names: []string{},
			Deprecated: false,
			Replacement: "" },
		PublicKey: pub,
		Comments:         []string{comment}	}
}

func SSHLoadJson(s []byte) Key {
	var key SSHKey
	json.Unmarshal(s, &key)

	return &key
}
