package data

import (
	"encoding/json"
	"testing"
	"time"
)

func TestKeyImplMerge(t *testing.T) {
	early := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		key   keyImpl
		other keyImpl
		check func(*testing.T, keyImpl)
	}{
		{
			name:  "names are unioned",
			key:   keyImpl{Names: setOf("a")},
			other: keyImpl{Names: setOf("b")},
			check: func(t *testing.T, k keyImpl) {
				if !k.Names.Contains("a") || !k.Names.Contains("b") {
					t.Errorf("names = %v, want both a and b", k.Names.StringArray())
				}
			},
		},
		{
			name:  "deprecation is sticky",
			key:   keyImpl{Deprecated: false},
			other: keyImpl{Deprecated: true},
			check: func(t *testing.T, k keyImpl) {
				if !k.Deprecated {
					t.Error("merging a deprecated key should deprecate the result")
				}
			},
		},
		{
			name:  "an already deprecated key stays deprecated",
			key:   keyImpl{Deprecated: true},
			other: keyImpl{Deprecated: false},
			check: func(t *testing.T, k keyImpl) {
				if !k.Deprecated {
					t.Error("merging should not undo deprecation")
				}
			},
		},
		{
			name:  "a missing replacement is filled in",
			key:   keyImpl{},
			other: keyImpl{Replacement: "newkey"},
			check: func(t *testing.T, k keyImpl) {
				if k.Replacement != "newkey" {
					t.Errorf("Replacement = %q, want newkey", k.Replacement)
				}
			},
		},
		{
			name:  "an existing replacement is not overwritten",
			key:   keyImpl{Replacement: "first"},
			other: keyImpl{Replacement: "second"},
			check: func(t *testing.T, k keyImpl) {
				if k.Replacement != "first" {
					t.Errorf("Replacement = %q, want the one already set", k.Replacement)
				}
			},
		},
		{
			name:  "the earliest sighting wins",
			key:   keyImpl{Earliest: late},
			other: keyImpl{Earliest: early},
			check: func(t *testing.T, k keyImpl) {
				if !k.Earliest.Equal(early) {
					t.Errorf("Earliest = %s, want %s", k.Earliest, early)
				}
			},
		},
		{
			name:  "a later sighting does not move the earliest",
			key:   keyImpl{Earliest: early},
			other: keyImpl{Earliest: late},
			check: func(t *testing.T, k keyImpl) {
				if !k.Earliest.Equal(early) {
					t.Errorf("Earliest = %s, want %s", k.Earliest, early)
				}
			},
		},
		{
			name:  "an unset date is taken from the other key",
			key:   keyImpl{},
			other: keyImpl{Earliest: late},
			check: func(t *testing.T, k keyImpl) {
				if !k.Earliest.Equal(late) {
					t.Errorf("Earliest = %s, want %s", k.Earliest, late)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.key
			other := tc.other
			key.Merge(&other)
			tc.check(t, key)
		})
	}
}

func setOf(values ...string) StringSet {
	s := StringSet{}
	s.AddArray(values)
	return s
}

func TestKeyImplExpire(t *testing.T) {
	key := keyImpl{}

	if key.IsDeprecated() {
		t.Error("a fresh key should not be deprecated")
	}

	key.Expire()

	if !key.IsDeprecated() {
		t.Error("Expire should deprecate the key")
	}
}

func TestKeyImplReplacementID(t *testing.T) {
	key := keyImpl{Replacement: "newkey"}

	if key.ReplacementID() != "newkey" {
		t.Errorf("ReplacementID() = %q, want newkey", key.ReplacementID())
	}
}

func TestNewKeyRecognisesContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantNil bool
	}{
		{"an ssh public key", testPublicKey + " someone@example.com", false},
		{"a PuTTY key is deliberately ignored", "PuTTY-User-Key-File-2: ssh-rsa", true},
		{"unrecognised content", "this is not a key at all", true},
		{"empty content", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewKey(tc.content, time.Now())
			if tc.wantNil && got != nil {
				t.Errorf("NewKey returned %v, want nil", got)
			}
			if !tc.wantNil && got == nil {
				t.Error("NewKey returned nil, want a key")
			}
		})
	}
}

func TestSSHKeyMergeUnionsCommentsAndIds(t *testing.T) {
	a := newTestSSHKey(t, "first@example.com")
	b := newTestSSHKey(t, "second@example.com")
	b.Ids.Add("extra-id")

	a.Merge(b)

	if !a.Comments.Contains("first@example.com") || !a.Comments.Contains("second@example.com") {
		t.Errorf("comments = %v, want both", a.Comments.StringArray())
	}
	if !a.Ids.Contains("extra-id") {
		t.Errorf("ids = %v, want the extra id carried over", a.Ids.Ids)
	}
}

func TestSSHKeyMergeRejectsOtherKeyTypes(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("merging a non-SSH key into an SSH key should panic")
		}
	}()

	key := newTestSSHKey(t, "someone@example.com")
	key.Merge(NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod"))
}

func TestSSHKeyKeyType(t *testing.T) {
	key := newTestSSHKey(t, "someone@example.com")

	if got := key.KeyType(); got != "ssh-rsa" {
		t.Errorf("KeyType() = %q, want ssh-rsa", got)
	}
}

func TestSSHKeyPublicKeyString(t *testing.T) {
	key := newTestSSHKey(t, "someone@example.com")

	if got := key.PublicKeyString(); len(got) == 0 {
		t.Error("PublicKeyString produced nothing")
	}
}

func TestSSHKeyJson(t *testing.T) {
	key := newTestSSHKey(t, "someone@example.com")

	bytes, err := key.Json()
	if err != nil {
		t.Fatalf("Json: %v", err)
	}

	restored := SSHLoadJson(bytes)
	if restored.Id() != key.Id() {
		t.Errorf("round-tripped key ID = %q, want %q", restored.Id(), key.Id())
	}
}

// A key known only by fingerprint has no public key material, and marshals with
// an explicit UNKNOWN marker that has to survive a round trip.
func TestPublicKeyRoundTripsUnknown(t *testing.T) {
	key := NewSSHKeyFromFingerprint("aws-keypair", time.Time{}, "ab:cd:ef")

	bytes, err := key.Json()
	if err != nil {
		t.Fatalf("Json: %v", err)
	}

	restored := new(SSHKey)
	if err := json.Unmarshal(bytes, restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if restored.PublicKey.Key != nil {
		t.Error("a key with no public key material should restore with none")
	}
	if restored.Id() != "ab:cd:ef" {
		t.Errorf("restored ID = %q, want ab:cd:ef", restored.Id())
	}
}

// Repository files are plain JSON that the README invites people to merge by
// hand in git. Malformed public key blocks must come back as errors rather
// than taking down the process.
func TestPublicKeyUnmarshalRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no type field", `{"Data":"AAAA"}`},
		{"type is not a string", `{"Type":42,"Data":"AAAA"}`},
		{"no data field", `{"Type":"ssh-rsa"}`},
		{"data is not a string", `{"Type":"ssh-rsa","Data":17}`},
		{"data is not base64", `{"Type":"ssh-rsa","Data":"not base64!!"}`},
		{"data is not a public key", `{"Type":"ssh-rsa","Data":"AAAA"}`},
		{"not an object at all", `"a bare string"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var p PublicKey
			if err := p.UnmarshalJSON([]byte(tc.input)); err == nil {
				t.Errorf("UnmarshalJSON(%s) succeeded, want an error", tc.input)
			}
		})
	}
}

func TestMergeIDArrays(t *testing.T) {
	tests := []struct {
		name string
		a, b []ID
		want []ID
	}{
		{"both empty", nil, nil, nil},
		{"only the first", []ID{"a"}, nil, []ID{"a"}},
		{"only the second", nil, []ID{"b"}, []ID{"b"}},
		{"disjoint", []ID{"a"}, []ID{"b"}, []ID{"a", "b"}},
		{"overlapping ids are deduplicated", []ID{"a", "b"}, []ID{"b", "c"}, []ID{"a", "b", "c"}},
		{"empty ids are dropped", []ID{"a", ""}, []ID{""}, []ID{"a"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeIDArrays(tc.a, tc.b)

			if len(got) != len(tc.want) {
				t.Fatalf("got %d ids %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}
			for _, want := range tc.want {
				found := false
				for _, g := range got {
					if g == want {
						found = true
					}
				}
				if !found {
					t.Errorf("missing %q in %v", want, got)
				}
			}
		})
	}
}

// A repository file can describe an SSH key with neither public key material
// nor any recorded fingerprint -- for instance after a bad hand-merge. Asking
// for its ID should not take the process down.
func TestSSHKeyIdWithNoIdentifiers(t *testing.T) {
	key := &SSHKey{keyImpl: keyImpl{Type: "SSHKey"}}

	if got := key.Id(); got != "" {
		t.Errorf("Id() = %q, want the empty ID", got)
	}
	if got := key.Identifiers(); len(got) != 0 {
		t.Errorf("Identifiers() = %v, want none", got)
	}
}
