package connection

import "testing"

func assertStringEquals(t *testing.T, s1, s2 string, message ...string) {
	if s1 != s2 {
		t.Error(message, "expected ", s1, " got ", s2)
	}
}

func TestBasename(t *testing.T) {
	assertStringEquals(t, "foo", basename("/path/to/foo"))
	assertStringEquals(t, "foo", basename("path/to/foo"))
	assertStringEquals(t, "foo", basename("foo"))
	assertStringEquals(t, "", basename(""))
}
