package data

import (
	"encoding/base64"
	"errors"
	"fmt"
)

type Fetcher interface {
	Fetch(id ID) (Key, error)
}

/** Where a Key is bound on an account
 */
type BindingLocation string

const (
	// UnspecifiedLocation is the zero value.  Bindings written before
	// locksmith recorded a location carry it, and it is claimed alongside
	// AUTHORIZED_KEYS by the SSH fetch so that those older records converge on
	// the first fetch after an upgrade rather than sitting beside the new ones
	// forever.
	UnspecifiedLocation BindingLocation = ""

	FILE                      BindingLocation = "FILE"
	AUTHORIZED_KEYS           BindingLocation = "AUTHORIZED_KEYS"
	AWS_CREDENTIALS           BindingLocation = "CREDENTIALS"
	INSTANCE_ROOT_CREDENTIALS BindingLocation = "INSTANCE ROOT"
)

type KeyBindingImpl struct {
	KeyID ID
	//AccountID ID `json:",omitempty"`
	Location BindingLocation `json:",omitempty"`
	Name     string          `json:",omitempty"`

	// Options is the authorized_keys option list that preceded the key on the
	// line it was found on -- `command="..."`, `from="..."`, `restrict`,
	// `no-pty` and friends -- stored verbatim, comma-separated, exactly as
	// `ssh.ParseAuthorizedKey` reports it.
	//
	// It belongs to the *binding*, not the key: the same key is routinely
	// unrestricted on one host and confined to a single command on another.
	//
	// It is load-bearing, not decoration.  A rendered line that omits it is a
	// privilege escalation: rotating a key confined to
	// `command="/usr/bin/rrsync -ro /srv"` would otherwise write its
	// replacement with no restriction at all, turning a read-only rsync into
	// an interactive shell, on every host the key was bound to.
	//
	// A string rather than a []string so that KeyBindingImpl stays comparable
	// and usable as a map key.  Joining on "," round-trips exactly, including
	// commas inside quoted option values.
	Options string `json:",omitempty"`
}

type KeyBinding interface {
	Describe(keylib Fetcher) (s string, key interface{})
	// TODO:  this should move into a speicfic binding type
	GetSshLine(keylib Fetcher) (string, error)
}

// Describe returns a key binding description and the key described
func (k *KeyBindingImpl) Describe(keylib Fetcher) (s string, key interface{}) {
	if k.Name != "" {
		s = k.Name + " = "
	}

	if found, err := keylib.Fetch(k.KeyID); err != nil {
		s = fmt.Sprintf("%s%s", s, "Unknown key "+k.KeyID)
	} else {
		s = fmt.Sprintf("%s%s", s, found)
		key = found
	}

	return
}

func (k *KeyBindingImpl) GetSshLine(keylib Fetcher) (string, error) {
	key, err := keylib.Fetch(k.KeyID)
	if err != nil {
		return "", err
	}

	sshKey, ok := key.(*SSHKey)
	if !ok {
		return "", errors.New(fmt.Sprint("Key ", key, " is not an SSH key"))
	}

	pub := sshKey.PublicKey.Key
	if pub == nil {
		return "", errors.New(fmt.Sprint("Key ", key, " has no public key material"))
	}

	line := fmt.Sprintf("%s %s",
		pub.Type(),
		base64.StdEncoding.EncodeToString(pub.Marshal()))

	// The trailing comment is optional in authorized_keys.
	if comments := sshKey.Comments.StringArray(); len(comments) > 0 {
		line = line + " " + comments[0]
	}

	// Restrictions go back on the front, or writing this line would silently
	// widen the key's privileges wherever it lands.
	if k.Options != "" {
		line = k.Options + " " + line
	}

	return line, nil
}

// PublicKeyBlob returns the base64 public key material of the bound key: the
// key's identity, independent of the comment and options that may accompany it
// on any given authorized_keys line.
//
// Both the add path's duplicate check and the remove path's sed address match
// on this rather than on a rendered line, because a rendered line carries a
// merged comment set that changes between runs.
func (k *KeyBindingImpl) PublicKeyBlob(keylib Fetcher) (string, error) {
	key, err := keylib.Fetch(k.KeyID)
	if err != nil {
		return "", err
	}

	sshKey, ok := key.(*SSHKey)
	if !ok {
		return "", errors.New(fmt.Sprint("Key ", key, " is not an SSH key"))
	}
	if sshKey.PublicKey.Key == nil {
		return "", errors.New(fmt.Sprint("Key ", key, " has no public key material"))
	}

	return base64.StdEncoding.EncodeToString(sshKey.PublicKey.Key.Marshal()), nil
}
