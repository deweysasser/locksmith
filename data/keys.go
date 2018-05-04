package data

import (
	"encoding/json"
	"github.com/deweysasser/locksmith/output"
	"io/ioutil"
	"strings"
	"time"
	"os"
	"fmt"
)

type keyImpl struct {
	Type        string
	Names       StringSet
	Deprecated  bool `json:",omitempty"`
	Replacement ID   `json:",omitempty"`
	Earliest    time.Time `json:",omitempty"`
}

type Key interface {
	Id() ID
	//IdString() string
	Identifiers() []ID
	GetNames() StringSet
	IsDeprecated() bool
	Expire()
	ReplacementID() ID
	Merge(Key)
}

func (key *keyImpl) StandardString(id ID, other ...string) string {
	ex := ""
	switch {
	case key.IsDeprecated():
		ex = "*EX*"
	case !key.Earliest.IsZero():
		dur := time.Since(key.Earliest)
		ex = formatAge(dur)
	}

	id2 := id
	if len(id) > 25 {
		id2 = id[:22] + "..."
	}

	return fmt.Sprintf("%6s %6s %-25.25s %s (%s)", key.Type, ex, id2, key.Names.Join(", "), strings.Join(other, ", "))
}

func formatAge(duration time.Duration) string {
	hours := int(duration.Hours())
	YEAR := 365*24
	WEEK := 7*24

	switch {
	case hours == 0:
		return ""
	case hours <= WEEK:
		return fmt.Sprintf("%dh", hours)
	case hours < YEAR:
		return fmt.Sprintf("%dw", hours/(WEEK))
	case hours%YEAR/WEEK == 0:
		return fmt.Sprintf("%dy", hours/YEAR)
	default:
		return fmt.Sprintf("%dy%02dw", hours/YEAR, (hours%YEAR)/WEEK, )
	}
	return ""
}

func (key *keyImpl) GetNames() StringSet {
	return key.Names
}

func (key *keyImpl) ReplacementID() ID {
	return key.Replacement
}

func (key *keyImpl) Expire() {
	key.Deprecated = true
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
	switch {
	case key.Earliest.IsZero():
		key.Earliest = k.Earliest
	case key.Earliest.After(k.Earliest):
		key.Earliest = k.Earliest
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
	if s, e := os.Stat(path); e == nil {
		bytes, err := ioutil.ReadFile(path)
		check(err)

		return NewKey(string(bytes), s.ModTime())
	} else {
		output.Error("Failed to read", path)
		return nil
	}
}

// Create a new Key from the given content
func NewKey(content string, t time.Time, names ...string) Key {

	switch {
	case strings.Contains(content, "PuTTY"):
		return nil
	case strings.Contains(content, "ssh-"):
		return parseSshPublicKey(content, t, names)
	case strings.Contains(content, "PRIVATE KEY"):
		output.Debug("Parsing private key from", names)
		return parseSshPrivateKey(content, t, names...)
	default:
		return nil
	}
}
