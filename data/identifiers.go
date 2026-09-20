package data

import (
	"crypto/sha256"
	"fmt"
)

type ID string

type Ider interface {
	Id() ID
}

type Identiferser interface {
	Identifiers() []ID
}

func IdFromString(s string) ID {
	return IdFromBytes([]byte(s))
}

func IdFromBytes(s []byte) ID {
	return ID(fmt.Sprintf("%x", sha256.Sum256(s)))
}

// Searchable is what an object offers a user searching for it, beyond its
// rendered form and its identifiers.
//
// It exists because the thing someone actually has in hand is usually none of
// those.  For an SSH key it is the base64 blob out of an authorized_keys line
// -- the key material itself, which is neither an identifier (those are
// fingerprints) nor rendered anywhere (it is 68 characters and the display
// truncates IDs at 22).
//
// Deliberately separate from Identifiers(): those key the library cache and
// name the file on disk, so adding a search term to them would change lookup
// and storage rather than just search.
type Searchable interface {
	SearchTerms() []string
}
