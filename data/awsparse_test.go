package data

import (
	"io/ioutil"
	"sort"
	"testing"
	"time"
)

// collectKeys drains ParseAWSCredentials into a slice.
func collectKeys(input []byte) []Key {
	keys := make(chan Key)
	go func() {
		defer close(keys)
		ParseAWSCredentials(input, keys)
	}()

	var got []Key
	for k := range keys {
		got = append(got, k)
	}
	return got
}

func keyIds(keys []Key) []string {
	var ids []string
	for _, k := range keys {
		ids = append(ids, string(k.Id()))
	}
	sort.Strings(ids)
	return ids
}

// hasName reports whether the key carries the given name. GetNames returns a
// StringSet by value and all of its methods take a pointer receiver, so the
// result has to be bound to a variable first.
func hasName(k Key, name string) bool {
	names := k.GetNames()
	return names.Contains(name)
}

func nameList(k Key) []string {
	names := k.GetNames()
	return names.StringArray()
}

func TestParseAWSCredentialsFixture(t *testing.T) {
	bytes, err := ioutil.ReadFile("test-data/credentials")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	got := keyIds(collectKeys(bytes))
	want := []string{"AKIAIQ5R76RH47DYH3OA", "AKIAIUNPBKF3ZP6MFSPQ"}

	if len(got) != len(want) {
		t.Fatalf("parsed %d keys %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("key[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// The profile name becomes the key's name, which is how `list` labels it.
func TestParseAWSCredentialsNamesKeysAfterProfile(t *testing.T) {
	input := []byte("[someone-prod]\naws_access_key_id = AKIAEXAMPLE\n")

	keys := collectKeys(input)
	if len(keys) != 1 {
		t.Fatalf("parsed %d keys, want 1", len(keys))
	}

	if !hasName(keys[0], "someone-prod") {
		t.Errorf("key names %v should include the profile name", nameList(keys[0]))
	}
}

// The secret is deliberately never carried off the disk.
func TestParseAWSCredentialsDropsTheSecret(t *testing.T) {
	input := []byte("[default]\naws_access_key_id = AKIAEXAMPLE\naws_secret_access_key = supersecret\n")

	keys := collectKeys(input)
	if len(keys) != 1 {
		t.Fatalf("parsed %d keys, want 1", len(keys))
	}

	awsKey, ok := keys[0].(*AWSKey)
	if !ok {
		t.Fatalf("key is %T, want *AWSKey", keys[0])
	}
	if awsKey.AwsSecretKey != "" {
		t.Errorf("secret key material leaked into the key: %q", awsKey.AwsSecretKey)
	}
}

func TestParseFile(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]map[string]string
	}{
		{
			name:  "empty input yields no sections",
			input: "",
			want:  map[string]map[string]string{},
		},
		{
			name:  "a section with no fields",
			input: "[default]\n",
			want:  map[string]map[string]string{"default": {}},
		},
		{
			name:  "values may be padded with spaces",
			input: "[default]\naws_access_key_id=  AKIA1\n",
			want:  map[string]map[string]string{"default": {"aws_access_key_id": "AKIA1"}},
		},
		{
			name:  "blank lines are ignored",
			input: "[default]\n\n\naws_access_key_id = AKIA1\n\n",
			want:  map[string]map[string]string{"default": {"aws_access_key_id": "AKIA1"}},
		},
		{
			name:  "multiple sections are kept apart",
			input: "[a]\nkey = 1\n[b]\nkey = 2\n",
			want:  map[string]map[string]string{"a": {"key": "1"}, "b": {"key": "2"}},
		},
		{
			name:  "a repeated section wins with its last definition",
			input: "[a]\nkey = 1\n[a]\nkey = 2\n",
			want:  map[string]map[string]string{"a": {"key": "2"}},
		},
		{
			// A credentials file that starts with a stray field, or one that
			// has been truncated mid-write, must not take the process down.
			name:  "fields before any section header are discarded",
			input: "orphan = value\n[default]\nkey = 1\n",
			want:  map[string]map[string]string{"default": {"key": "1"}},
		},
		{
			name:  "a file of nothing but orphan fields yields no sections",
			input: "orphan = value\nanother = thing\n",
			want:  map[string]map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseFile(tc.input)

			if len(got) != len(tc.want) {
				t.Fatalf("parsed %v, want %v", got, tc.want)
			}
			for section, fields := range tc.want {
				gotFields, ok := got[section]
				if !ok {
					t.Fatalf("missing section %q in %v", section, got)
				}
				if len(gotFields) != len(fields) {
					t.Fatalf("section %q = %v, want %v", section, gotFields, fields)
				}
				for k, v := range fields {
					if gotFields[k] != v {
						t.Errorf("section %q field %q = %q, want %q", section, k, gotFields[k], v)
					}
				}
			}
		})
	}
}

// A section without an access key id would otherwise yield a key with an empty
// ID, which collides with every other such key in the library.
func TestParseAWSCredentialsSkipsSectionsWithoutAKeyId(t *testing.T) {
	input := []byte("[no-key-here]\nregion = us-east-1\n[real]\naws_access_key_id = AKIAREAL\n")

	got := keyIds(collectKeys(input))
	if len(got) != 1 || got[0] != "AKIAREAL" {
		t.Errorf("parsed keys %v, want only AKIAREAL", got)
	}
}

func TestNewAwsKey(t *testing.T) {
	created := time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
	key := NewAwsKey("AKIAEXAMPLE", created, true, "prod", "")

	if key.Id() != "AKIAEXAMPLE" {
		t.Errorf("Id() = %q, want AKIAEXAMPLE", key.Id())
	}
	if ids := key.Identifiers(); len(ids) != 1 || ids[0] != "AKIAEXAMPLE" {
		t.Errorf("Identifiers() = %v, want [AKIAEXAMPLE]", ids)
	}
	if !key.Active {
		t.Error("key should be active")
	}
	if key.IsDeprecated() {
		t.Error("a newly discovered key should not be deprecated")
	}
	if !hasName(key, "prod") {
		t.Errorf("names %v should contain prod", nameList(key))
	}
	if hasName(key, "") {
		t.Errorf("names %v should not contain the empty string", nameList(key))
	}
	if ids := key.Ids(); len(ids) != 1 || ids[0] != "AKIAEXAMPLE" {
		t.Errorf("Ids() = %v, want [AKIAEXAMPLE]", ids)
	}
}

func TestAWSKeyMerge(t *testing.T) {
	early := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

	key := NewAwsKey("AKIAEXAMPLE", late, true, "from-iam")
	other := NewAwsKey("AKIAEXAMPLE", early, true, "from-credentials-file")
	other.Expire()

	key.Merge(other)

	if !hasName(key, "from-iam") || !hasName(key, "from-credentials-file") {
		t.Errorf("merged names %v should hold both sources", nameList(key))
	}
	if !key.IsDeprecated() {
		t.Error("merging a deprecated key should deprecate the result")
	}
	if !key.Earliest.Equal(early) {
		t.Errorf("Earliest = %s, want the earlier of the two (%s)", key.Earliest, early)
	}
}

// Merging a key of another type is a no-op rather than a panic.
func TestAWSKeyMergeIgnoresOtherKeyTypes(t *testing.T) {
	key := NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
	sshKey := NewSSHKeyFromFingerprint("other", time.Time{}, "id1")

	key.Merge(sshKey)

	if !hasName(key, "prod") {
		t.Errorf("names %v should be untouched", nameList(key))
	}
}

func TestAWSKeyJson(t *testing.T) {
	key := NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")

	bytes, err := key.Json()
	if err != nil {
		t.Fatalf("Json: %v", err)
	}
	if len(bytes) == 0 {
		t.Error("Json produced no output")
	}
}
