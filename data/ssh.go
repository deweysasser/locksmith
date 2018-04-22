package data

import (
	//	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
	"strings"
	"encoding/base64"
)

type PublicKey struct {
	Key ssh.PublicKey
}


func (p *PublicKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type, Data string
	}{
		Type: p.Key.Type(),
		Data: base64.StdEncoding.EncodeToString(p.Key.Marshal()),
	})
}

func (p *PublicKey) UnmarshalJSON(bytes []byte) error {
	temp := make(map[string]interface{})
	if e := json.Unmarshal(bytes, &temp); e != nil {
		return e
	}

	if bKey, e := base64.StdEncoding.DecodeString(temp["Data"].(string)); e==nil {
		if k, e2 := ssh.ParsePublicKey(bKey); e2 == nil {
			p.Key = k
			return nil
		} else {
			return e2
		}
	} else {
		return e
	}
}

/** An SSH Key, public and (optionally) private
 */
type SSHKey struct {
	keyImpl
	ids []ID
	PublicKey PublicKey
	Comments  []string
}

/* Use of a public Key, e.g. in an authorized_keys file
 */
type SSHBinding struct {
	Id      ID
	Comment string
	Options []string
}

func NewSshKey(pub ssh.PublicKey) *SSHKey {
	return &SSHKey{keyImpl{"SSHKey",[]string{}, false, ""},nil,PublicKey{pub}, []string{},}
}

func (key *SSHKey) Id() ID {
	return key.Identifiers()[0]
}

func (key *SSHKey) Identifiers() []ID {
	if key.ids == nil {
		key.ids = append(key.ids, ID(ssh.FingerprintSHA256(key.PublicKey.Key)))
		key.ids = append(key.ids, ID(ssh.FingerprintLegacyMD5(key.PublicKey.Key)))
	}

	return key.ids
}

func (key *SSHKey) String() string {
	return fmt.Sprintf("%s %s %s", key.Type, key.Id(), strings.Join(key.Comments, ", "))
}

func (key *SSHKey) KeyType() string {
	return key.PublicKey.Key.Type()
}

func (key *SSHKey) Json() ([]byte, error) {
	return json.MarshalIndent(key, "", "  ")
}

func (key *SSHKey) IsDeprecated() bool {
	return false
}

func (key *SSHKey) Replacement() ID {
	return ""
}

func publicKey(keytype, pub string) ssh.PublicKey {
	line := fmt.Sprintf("%s %s", keytype, pub)
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	check(err)
	return pubkey
}

func (key *SSHKey) PublicKeyString() string {
	return string(ssh.MarshalAuthorizedKey(key.PublicKey.Key))
}

func getId(pub ssh.PublicKey) ID {
	return ID(ssh.FingerprintSHA256(pub))
}

func (key *SSHKey) GetNames() []string {
	return key.Names
}

func parseSshPrivateKey(content string) Key {
	signer, err := ssh.ParsePrivateKey([]byte(content))
	check(err)
	pub := signer.PublicKey()
	return &SSHKey{
		keyImpl: keyImpl{
			Type:        "SSHKey",
			Names:       []string{},
			Deprecated:  false,
			Replacement: ""},
		PublicKey: PublicKey{pub},
		Comments:  []string{}}
}

func parseSshPublicKey(content string) Key {
	//	pub, comment, options, _, err := ssh.ParseAuthorizedKey([]byte(content))
	pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(content))
	check(err)
	return &SSHKey{
		keyImpl: keyImpl{
			Type:        "SSHKey",
			Names:       []string{},
			Deprecated:  false,
			Replacement: ""},
		PublicKey: PublicKey{pub},
		Comments:  []string{comment}}
}


func SSHLoadJson(s []byte) Key {
	var key SSHKey
	json.Unmarshal(s, &key)

	return &key
}
