package data

import (
	"testing"
)

func TestBasicKeys(t *testing.T) {
	var id ID

	id = "foo"

	assertStringsEquals(t, "foo", string(id))
}
