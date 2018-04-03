package keys

import (
	"testing"
)

func TestBasic(t *testing.T) {
	impl := NewAwsKey("12345", "")

	var key Key
	key = impl

	assertStringsEquals(t, "12345", key.Id())

}
