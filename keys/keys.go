package keys

import "io/ioutil"
import "strings"

type Key interface{}

type GenericKeyImpl struct {
	Value string
}

type SshKeyImpl struct {
	Type, KeyValue, Comment string
}

// Create a new key from the given path
func Read(path string) Key {
	bytes, err := ioutil.ReadFile(path)
	check(err)

	return New(string(bytes))
}

func ReadMany(path string) []Key {
     return nil
}

// Create a new key from the given content
func New(content string) Key {
	switch {
	case strings.Contains(content, "ssh-"):
		return parseSshPublicKey(content)
	default:
		return nil
	}
}
