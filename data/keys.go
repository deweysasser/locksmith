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


type Key interface {
	Id() ID
	//IdString() string
	Identifiers() []ID
	GetNames() StringSet
	IsDeprecated() bool
	ReplacementID() ID
	Merge(Key)
}

func (key *keyImpl) GetNames() StringSet {
	return key.Names
}

func (key *keyImpl) ReplacementID() ID {
	return key.Replacement
}

func (key *keyImpl) IsDeprecated() bool {
	return key.Deprecated
}


func (key *keyImpl) Merge(k *keyImpl) {
	key.Names.AddSet(k.Names)
	key.Deprecated = key.Deprecated || k.Deprecated
	if key.Replacement == "" && k.Replacement != "" {
		key.Replacement = k.Replacement
	}
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

	return NewKey(string(bytes))
}

// Create a new Key from the given content
func NewKey(content string, names ...string) Key {

	switch {
	case strings.Contains(content, "PuTTY"):
		return nil
	case strings.Contains(content, "ssh-"):
		return parseSshPublicKey(content, names...)
	/*
		case strings.Contains(content, "PRIVATE KEY"):
			return parseSshPrivateKey(content)
	*/
	default:
		return nil
	}
}
