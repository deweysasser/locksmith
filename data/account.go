package data

import (
	"encoding/json"
	"fmt"
)

type accountImpl struct {
	Type       string
	Name       string
	Connection ID
	Keys       []KeyBinding
}

type SSHAccount struct {
	accountImpl
}

type AWSAccount struct {
	accountImpl
}

type Account interface {
	Ider
	Bindings() []KeyBinding
	Merge(a Account)
}

func NewSSHAccount(name string, connID ID, keys []KeyBinding) *SSHAccount {
	return &SSHAccount{accountImpl{"SSHAccount", name, connID, keys}}
}

func NewAWSAccount(name string, connID ID, keys []KeyBinding) *AWSAccount {
	return &AWSAccount{accountImpl{"AWSAccount", name, connID, keys}}
}

func (a *accountImpl) Merge(account accountImpl) {
	a.Keys = mergeBindings(a.Keys, account.Keys)
}

func (a *SSHAccount) Merge(account Account) {
	a.accountImpl.Merge(account.(*SSHAccount).accountImpl)
}

func (a *SSHAccount) String() string {
	return fmt.Sprintf("SSH %s", a.Name)
}

func (a *AWSAccount) Merge(account Account) {
	a.accountImpl.Merge(account.(*AWSAccount).accountImpl)
}

func (a *AWSAccount) String() string {
	return fmt.Sprintf("aws %s", a.Name)
}

func (a *accountImpl) Bindings() []KeyBinding {
	return a.Keys
}

func (a *accountImpl) String() string {
	return fmt.Sprintf("%s", a.Name)
}

func (a *accountImpl) AddBinding(k Key) {
	a.Keys = append(a.Keys, KeyBinding{KeyID: k.Id() /* AccountID: a.Id() */})
}

func (a *accountImpl) Id() ID {
	return ID(a.Type + a.Name)
}

// mergeBindings merges 2 arrays of keybindings resulting in an array of unique keyBindings
// The implementation is a bit of a hack right now
func mergeBindings(b1 []KeyBinding, b2 []KeyBinding) []KeyBinding {
	s := StringSet{}
	for _, k := range b1 {
		s.Add(toJson(&k))
	}

	for _, k := range b2 {
		s.Add(toJson(&k))
	}

	var result []KeyBinding

	for s := range s.Values() {
		result = append(result, fromJson(s))
	}

	return result
}

func toJson(binding *KeyBinding) string {
	if bytes, err := json.Marshal(binding); err == nil {
		return string(bytes)
	} else {
		panic(fmt.Sprintf("Failed to jsonify keybinding: %s", err))
	}
}

func fromJson(s string) KeyBinding {
	var k KeyBinding

	if e := json.Unmarshal([]byte(s), &k); e == nil {
		return k
	} else {
		panic(fmt.Sprintf("Failed to unmarshal keybinding: %s", e))
	}
}
