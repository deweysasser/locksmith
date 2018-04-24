package data

import (
	"encoding/json"
	"fmt"
)

type Account struct {
	Type       string
	Name       string
	Connection ID
	Keys       []KeyBinding
}

func (a *Account) String() string {
	return fmt.Sprintf("Account %s", a.Name)
}

func (a *Account) AddBinding(k Key) {
	a.Keys = append(a.Keys, KeyBinding{KeyID: k.Id(), /* AccountID: a.Id() */})
}

func (a *Account) Id() ID {
	return IdFromString(a.Name)
}

func LoadAccount(bytes []byte) (*Account, error) {
	a := new(Account)

	e := json.Unmarshal(bytes, &a)

	if e == nil {
		return a, nil
	}

	return nil, e
}
