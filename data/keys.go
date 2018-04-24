package data

import (
	"encoding/json"
	"io/ioutil"
	"strings"
)

type keyImpl struct {
	Type        string
	Names       StringSet
	Deprecated  bool `json:",omitempty"`
	Replacement ID `json:",omitempty"`
}

/** What action to perform (if any) for a binding
 */
type BindingAction string

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

type Key interface {
	Id() ID
	//IdString() string
	Identifiers() []ID
	GetNames() []string
	IsDeprecated() bool
	ReplacementID() ID
	Merge(Key)
}

func (key *keyImpl) GetNames() []string {
	return key.Names.StringArray()
}

func (key *keyImpl) ReplacementID() ID {
	return key.Replacement
}

func LoadTypeFromJSON(s []byte, o Key) Key {
	json.Unmarshal(s, o)

	return o
}


func LoadJsonFile(path string) Key {
	json, e := ioutil.ReadFile(path)
	check(e)

	return SSHLoadJson(json)
}

// Create a new Key from the given path
func Read(path string) Key {
	bytes, err := ioutil.ReadFile(path)
	check(err)

	return New(string(bytes))
}

// Create a new Key from the given content
func New(content string) Key {
	switch {
	case strings.Contains(content, "PuTTY"):
		return nil
	case strings.Contains(content, "ssh-"):
		return parseSshPublicKey(content)
	/*
		case strings.Contains(content, "PRIVATE KEY"):
			return parseSshPrivateKey(content)
	*/
	default:
		return nil
	}
}
