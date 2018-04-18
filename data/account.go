package data

import "encoding/json"

type Account struct {
	Type string
	Name string
	//Keys []KeyBinding
}

func LoadAccount(bytes []byte) (*Account, error) {
	a := new(Account)

	e := json.Unmarshal(bytes, &a)

	if e == nil {
		return a, nil
	}

	return nil, e
}

