package keys

import "io/ioutil"
import "strings"

type KeyID string

type keyImpl struct {
	Type        string
	Ids         []KeyID
	Names       []string
	Deprecated  bool
	Replacement KeyID
}

type Key interface {
	Json() ([]byte, error)
	Id() KeyID
	Identifiers() []KeyID
	GetNames() []string
	IsDeprecated() bool
	ReplacementID() KeyID
}

func (key *keyImpl) Id() KeyID {
	return key.Ids[0]
}


func (key *keyImpl) Identifiers() []KeyID {
	return key.Ids
}

func (key *keyImpl) ReplacementID() KeyID {
	return key.Replacement
}

// Load the key from the given JSON
func ReadJson(json []byte) Key {
	// For now, SSH key is the only kind
	return SSHLoadJson(json)
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
