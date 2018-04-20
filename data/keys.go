package data

import (
	"encoding/json"
	"io/ioutil"
	"strings"
)


type keyImpl struct {
	Type        string
	Ids         []ID
	Names       []string
	Deprecated  bool
	Replacement ID
}

type BindingType int

const(
	EXISTS BindingType = iota
	PENDING_ADD BindingType = iota
	PENDING_DELETE BindingType = iota
)

type KeyBinding struct {
	KeyID     ID
	AccountID ID
	Type      BindingType
}

type Key interface {
	Json() ([]byte, error)
	Id() ID
	Identifiers() []ID
	GetNames() []string
	IsDeprecated() bool
	ReplacementID() ID
}

func (key *keyImpl) Id() ID {
	return key.Ids[0]
}

func (key *keyImpl) GetNames() []string {
	return key.Names
}


func (key *keyImpl) Identifiers() []ID {
	return key.Ids
}

func (key *keyImpl) ReplacementID() ID {
	return key.Replacement
}


// Load the key from the given JSON
func ReadJson(bytes []byte) Key {
	var key keyImpl
	json.Unmarshal(bytes, &key)
	
	switch key.Type {
	// For now, SSH key is the only kind
	case "SSHKey":
		return SSHLoadJson(bytes)
	}
	return nil
}

func LoadJsonFile(path string) Key {
	json, e := ioutil.ReadFile(path)
	check(e)

	return SSHLoadJson(json)
}

// Create a new key from the given path
func Read(path string) Key {
	bytes, err := ioutil.ReadFile(path)
	check(err)

	return New(string(bytes))
}

// Create a new key from the given content
func New(content string) Key {
	switch {
	case strings.Contains(content, "ssh-"):
		return parseSshPublicKey(content)
	case strings.Contains(content, "PRIVATE KEY"):
		return parseSshPrivateKey(content)
	default:
		return nil
	}
}
