package data

import (
	"encoding/json"
	"fmt"
	"github.com/deweysasser/locksmith/output"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

type keyImpl struct {
	Type        string
	Names       StringSet
	Deprecated  bool      `json:",omitempty"`
	Replacement ID        `json:",omitempty"`
	Earliest    time.Time `json:",omitempty"`
}

type Key interface {
	Id() ID
	//IdString() string
	Identifiers() []ID
	GetNames() StringSet
	IsDeprecated() bool
	Unexpire()
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

	return fmt.Sprintf("%6s %9s %-25.25s %s (%s)", key.Type, ex, id2, key.Names.Join(", "), strings.Join(other, ", "))
}

func formatAge(duration time.Duration) string {
	const (
		WEEK = 7 * 24
		YEAR = 365 * 24
	)

	// Anything under a minute, including a timestamp that is somehow in the
	// future, reads as current.  StandardString renders an *unknown* date as
	// blank, so this must look different from blank.
	if duration < time.Minute {
		return "right now"
	}

	if minutes := int(duration.Minutes()); minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}

	hours := int(duration.Hours())

	switch {
	case hours <= WEEK:
		return fmt.Sprintf("%dh", hours)
	case hours < YEAR:
		return fmt.Sprintf("%dw", hours/WEEK)
	case hours%YEAR/WEEK == 0:
		return fmt.Sprintf("%dy", hours/YEAR)
	default:
		return fmt.Sprintf("%dy%02dw", hours/YEAR, (hours%YEAR)/WEEK)
	}
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

// Unexpire clears the legacy Deprecated flag.
//
// Intent now lives in a data.KeyPolicy rather than on the key record, but
// repositories written by an older locksmith carry the flag here, and it has to
// be clearable or `unexpire` could not undo what `expire` used to do.
func (key *keyImpl) Unexpire() {
	key.Deprecated = false
	key.Replacement = ""
}

func (key *keyImpl) Merge(k *keyImpl) {
	key.Names.AddSet(k.Names)
	key.Deprecated = key.Deprecated || k.Deprecated
	if key.Replacement == "" && k.Replacement != "" {
		key.Replacement = k.Replacement
	}
	switch {
	case k.Earliest.IsZero():
		// This sighting offers no date -- several sources have none to give,
		// Digital Ocean's key API among them.  Keep whatever we already knew;
		// otherwise the zero time, being year 1, wins every comparison and
		// destroys a real timestamp learned from somewhere else.
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

// NewKeys reads every key in content.
//
// A public key file may legitimately hold many entries -- an authorized_keys
// file is the whole point of this tool -- and ssh.ParseAuthorizedKey returns
// only the first, discarding the rest.  NewKey therefore sees one key where a
// file holds five, and the other four are silently never catalogued.  Anything
// reading a whole file should use this instead.
//
// A private key file is a single key by construction, so that case is passed
// through to NewKey whole.
func NewKeys(content string, t time.Time, names ...string) []Key {
	switch {
	case strings.Contains(content, "PuTTY"):
		return nil
	case strings.Contains(content, "PRIVATE KEY"):
		if key := NewKey(content, t, names...); key != nil {
			return []Key{key}
		}
		return nil
	}

	var keys []Key
	for _, line := range strings.Split(content, "\n") {
		if !looksLikeSSHPublicKey(line) {
			continue
		}
		if key := NewKey(line, t, names...); key != nil {
			keys = append(keys, key)
		}
	}

	return keys
}

// Create a new Key from the given content
func NewKey(content string, t time.Time, names ...string) Key {

	switch {
	case strings.Contains(content, "PuTTY"):
		// PuTTY's native format is not something we can read.
		return nil
	case strings.Contains(content, "PRIVATE KEY"):
		output.Debug("Parsing private key from", names)
		return parseSshPrivateKey(content, t, names...)
	case looksLikeSSHPublicKey(content):
		return parseSshPublicKey(content, t, names)
	default:
		return nil
	}
}
