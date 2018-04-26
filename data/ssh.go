package data

import (
	//	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
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
	Comments  StringSet
}

/* Use of a public Key, e.g. in an authorized_keys file
 */
type SSHBinding struct {
	Id      ID
	Comment string
	Options []string
}

func (s *SSHKey) Merge(k Key) {
	if other, ok := k.(*SSHKey); ok {
	   s.Deprecated = s.Deprecated || other.Deprecated
	   s.Names.AddSet(other.Names)
	   s.Comments.AddSet(other.Comments)
	} else {
		panic("SSH asked to merge non-SSH key")
	}
}

func NewSshKey(pub ssh.PublicKey) *SSHKey {
	return &SSHKey{keyImpl{"SSHKey",StringSet{}, false, ""},nil,PublicKey{pub}, StringSet{},}
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
	return fmt.Sprintf("%s %s %s (%s)", key.Type, key.Id(), key.Comments.Join(", "), key.Names.Join(", "))
}

func (key *SSHKey) KeyType() string {
	return key.PublicKey.Key.Type()
}

func (key *SSHKey) Json() ([]byte, error) {
	return json.MarshalIndent(key, "", "  ")
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

func parseSshPrivateKey(content string) Key {
	signer, err := ssh.ParsePrivateKey([]byte(content))
	check(err)
	pub := signer.PublicKey()
	return &SSHKey{
		keyImpl: keyImpl{
			Type:        "SSHKey",
			Names:       StringSet{},
			Deprecated:  false,
			Replacement: ""},
		PublicKey: PublicKey{pub},
		Comments:  StringSet{},
	}
}

func parseSshPublicKey(content string, names ...string) Key {
	//	pub, comment, options, _, err := ssh.ParseAuthorizedKey([]byte(content))
	pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(content))
	comments := StringSet{}
	if comment != "" {
		comments.Add(comment)
	}

	sNames := StringSet{}
	for _, s:= range(names) {
		if s != "" {
			sNames.Add(s)
		}
	}


	check(err)
	return &SSHKey{
		keyImpl: keyImpl{
			Type:        "SSHKey",
			Names:       sNames,
			Deprecated:  false,
			Replacement: ""},
		PublicKey: PublicKey{pub},
		Comments:  comments,
	}
}


func SSHLoadJson(s []byte) Key {
	key := new(SSHKey)
	json.Unmarshal(s, key)

	return key
}
