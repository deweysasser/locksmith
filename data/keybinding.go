package data

import (
	"encoding/base64"
	"errors"
	"fmt"
)

type Fetcher interface {
	Fetch(id ID) (Key, error)
}

/** Where a Key is bound on an account
 */
type BindingLocation string

const (
	FILE                      BindingLocation = "FILE"
	AUTHORIZED_KEYS           BindingLocation = "AUTHORIZED_KEYS"
	AWS_CREDENTIALS           BindingLocation = "CREDENTIALS"
	INSTANCE_ROOT_CREDENTIALS BindingLocation = "INSTANCE ROOT"
)

type KeyBindingImpl struct {
	KeyID ID
	//AccountID ID `json:",omitempty"`
	Location BindingLocation `json:",omitempty"`
	Name     string          `json:",omitempty"`
}

type KeyBinding interface {
	Describe(keylib Fetcher) (s string, key interface{})
	// TODO:  this should move into a speicfic binding type
	GetSshLine(keylib Fetcher) (string, error)
}

// Describe returns a key binding description and the key described
func (k *KeyBindingImpl) Describe(keylib Fetcher) (s string, key interface{}) {
	if k.Name != "" {
		s = k.Name + " = "
	}

	if found, err := keylib.Fetch(k.KeyID); err != nil {
		s = fmt.Sprintf("%s%s", s, "Unknown key "+k.KeyID)
	} else {
		s = fmt.Sprintf("%s%s", s, found)
		key = found
	}

	return
}

func (k *KeyBindingImpl) GetSshLine(keylib Fetcher) (string, error) {
	key, err := keylib.Fetch(k.KeyID)
	if err != nil {
		return "", err
	}

	sshKey, ok := key.(*SSHKey)
	if !ok {
		return "", errors.New(fmt.Sprint("Key ", key, " is not an SSH key"))
	}

	pub := sshKey.PublicKey.Key
	if pub == nil {
		return "", errors.New(fmt.Sprint("Key ", key, " has no public key material"))
	}

	line := fmt.Sprintf("%s %s",
		pub.Type(),
		base64.StdEncoding.EncodeToString(pub.Marshal()))

	// The trailing comment is optional in authorized_keys.
	if comments := sshKey.Comments.StringArray(); len(comments) > 0 {
		line = line + " " + comments[0]
	}

	return line, nil
}
