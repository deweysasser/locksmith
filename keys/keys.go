package keys

import "io/ioutil"
import "strings"

type Key interface {
	Json() ([]byte, error)
	Id() string
}

type GenericKeyImpl struct {
	Value string
}

type SshKeyImpl struct {
	Type, KeyValue, Comment string
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