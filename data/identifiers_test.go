package data

import "testing"

// Connection IDs are derived from the connection string, so the hash has to
// stay stable across releases or every stored connection changes filename.
func TestIdFromString(t *testing.T) {
	tests := []struct {
		in   string
		want ID
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	}

	for _, tc := range tests {
		if got := IdFromString(tc.in); got != tc.want {
			t.Errorf("IdFromString(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIdFromStringAndBytesAgree(t *testing.T) {
	if IdFromString("hello") != IdFromBytes([]byte("hello")) {
		t.Error("IdFromString and IdFromBytes should produce the same ID")
	}
}

func TestIdFromStringDistinguishesInputs(t *testing.T) {
	if IdFromString("a") == IdFromString("b") {
		t.Error("different inputs should produce different IDs")
	}
}
