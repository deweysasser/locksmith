package data

import (
	"testing"
)

func TestBasic(t *testing.T) {
	impl := NewAwsKey("12345", "")

	var key Key
	key = impl

	assertStringsEquals(t, "12345", string(key.Id()))

}
