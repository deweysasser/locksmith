	package data

import (
	"fmt"
)

/** What action to perform (if any) for a binding
 */
type BindingAction string

type Fetcher interface {
	Fetch(id string) (interface{}, error)
}

const (
	EXISTS         BindingAction = ""
	PENDING_ADD    BindingAction = "ADD"
	PENDING_DELETE BindingAction = "REMOVE"
)

/** Where a Key is bound on an account
 */
type BindingLocation string

const (
	FILE            BindingLocation = "FILE"
	AUTHORIZED_KEYS BindingLocation = "AUTHORIZED_KEYS"
	AWS_CREDENTIALS BindingLocation = "CREDENTIALS"
)

type KeyBinding struct {
	KeyID     ID
	//AccountID ID `json:",omitempty"`
	Location  BindingLocation `json:",omitempty"`
	Type      BindingAction `json:",omitempty"`
	Name      string `json:",omitempty"`
}


func (k *KeyBinding) Describe(keylib Fetcher) (s string, key interface{}) {
	if k.Name != "" {
		s = k.Name + " = "
	}

	if	key, err := keylib.Fetch(string(k.KeyID)); err != nil {
			s = fmt.Sprintf("%s%s", s, "Unknown key " + k.KeyID)
	} else {
		s = fmt.Sprintf("%s%s", s, key)
	}

	return
}
