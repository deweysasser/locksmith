package keys

import (
	"testing"
)

func TestBasicKeys(t *testing.T) {
	var id KeyID

	id = "foo"

	assertStringsEquals(t, "foo", string(id))
}
