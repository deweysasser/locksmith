package data

import "fmt"

// A KeyPolicy is a statement of intent about a key: what should become of it,
// wherever it is found.
//
// Intent used to live on the key record itself, as a Deprecated flag and a
// Replacement ID set by `expire`.  That conflated two different things.  A key
// record is an *observation* -- this key exists, here is where it was seen --
// and observations are re-merged from reality on every fetch.  Intent is not an
// observation, and storing it on the observed object had three consequences:
//
//   - It could not be revoked.  Merge only ever ORs Deprecated on, so there was
//     no way to un-expire a key short of editing JSON by hand.
//   - It survived every re-fetch, whether or not it still made sense.
//   - `plan` could not be a function of anything, because its inputs changed
//     underneath it.
//
// A policy is a separate, durable object keyed by the key it talks about.
// Revoking intent is deleting a file.
type KeyPolicy struct {
	Type        string
	KeyID       ID
	Disposition Disposition
	// Replacement is the key that should take this one's place, for
	// DispositionReplace.
	Replacement ID `json:",omitempty"`
}

// Disposition is what should happen to a key.
type Disposition string

const (
	// DispositionRemove: this key should not be authorized anywhere.
	DispositionRemove Disposition = "remove"

	// DispositionReplace: wherever this key is bound, its replacement should be
	// bound instead.
	DispositionReplace Disposition = "replace"
)

// NewRemovePolicy states that a key should be taken off every system.
func NewRemovePolicy(key ID) KeyPolicy {
	return KeyPolicy{
		Type:        "KeyPolicy",
		KeyID:       key,
		Disposition: DispositionRemove,
	}
}

// NewReplacePolicy states that a key should be replaced by another wherever it
// is found.
func NewReplacePolicy(key, replacement ID) KeyPolicy {
	return KeyPolicy{
		Type:        "KeyPolicy",
		KeyID:       key,
		Disposition: DispositionReplace,
		Replacement: replacement,
	}
}

// Id is on the value receiver for the same reason data.Change.Id is: policies
// are stored and listed by value, and only a value receiver puts Id in the
// method set of both KeyPolicy and *KeyPolicy.  Without it the library cannot
// see a policy as a data.Ider and silently keys it by a hash of its JSON
// instead of by the key it concerns.
func (p KeyPolicy) Id() ID {
	return p.KeyID
}

// Removes reports whether this policy means the key should come off systems.
// Both dispositions do: replacing a key removes the old one.
func (p KeyPolicy) Removes() bool {
	return p.Disposition == DispositionRemove || p.Disposition == DispositionReplace
}

func (p KeyPolicy) String() string {
	switch p.Disposition {
	case DispositionReplace:
		return fmt.Sprintf("replace %s with %s", p.KeyID, p.Replacement)
	case DispositionRemove:
		return fmt.Sprintf("remove %s", p.KeyID)
	default:
		return fmt.Sprintf("unknown policy %q for %s", p.Disposition, p.KeyID)
	}
}
