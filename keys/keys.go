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
	Ids() []string
	GetNames() []string
	IsDeprecated() bool
	Replacement() KeyID
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
