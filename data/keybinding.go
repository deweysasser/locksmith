package data

import (
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

type KeyBinding struct {
	KeyID ID
	//AccountID ID `json:",omitempty"`
	Location BindingLocation `json:",omitempty"`
	Name     string          `json:",omitempty"`
}

// Describe returns a key binding description and the key described
func (k *KeyBinding) Describe(keylib Fetcher) (s string, key interface{}) {
	if k.Name != "" {
		s = k.Name + " = "
	}

	if key, err := keylib.Fetch(k.KeyID); err != nil {
		s = fmt.Sprintf("%s%s", s, "Unknown key "+k.KeyID)
	} else {
		s = fmt.Sprintf("%s%s", s, key)
	}

	return
}
