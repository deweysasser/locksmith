package data

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeFetcher is a stand-in for lib.KeyLibrary, which data/ cannot import.
type fakeFetcher map[ID]Key

func (f fakeFetcher) Fetch(id ID) (Key, error) {
	if k, ok := f[id]; ok {
		return k, nil
	}
	return nil, errors.New("no such key " + string(id))
}

// testPublicKey is a throwaway key used to exercise binding rendering.
const testPublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDOzO+CaGCJLpMKrn2cqxb0Ln4L8SfKKG/qJfMNVUIFS8Xrqmw1DHHDLqSUvcvj1fxMtxwFI3JIrm4WHqCJCZ5aQKm0h4mOxnb0j/fZIXIUv8vp1bdCwJLKqcGF4/K1RQXJoTKPS0uLJrUW8rIX7YgZqZNbGfmKYBeCqAJfPMbHJDMIm5cYwRCAvwFmCn2N8w1UYs1U8lOgQyPFbzTLGO/VRbKcCFvCXLTKGvMQqVD3VwqKmxdmXKQKWO2PcSrLkFQeBqjX2fLxJzUDKGH0ZJ1mzQnDnU4y2hRbMZsWJYD8rPVJLMkNQbMZpVVZqGGXhWTYzFZnMRhNQQsRZaQV4z7B"

func newTestSSHKey(t *testing.T, comment string) *SSHKey {
	t.Helper()
	line := testPublicKey
	if comment != "" {
		line = line + " " + comment
	}
	k := NewKey(line, time.Now())
	if k == nil {
		t.Fatalf("failed to parse the test public key")
	}
	sshKey, ok := k.(*SSHKey)
	if !ok {
		t.Fatalf("parsed key is %T, want *SSHKey", k)
	}
	return sshKey
}

func TestDescribeKnownKey(t *testing.T) {
	key := newTestSSHKey(t, "someone@example.com")
	lib := fakeFetcher{key.Id(): key}
	binding := KeyBindingImpl{KeyID: key.Id(), Location: AUTHORIZED_KEYS}

	s, described := binding.Describe(lib)

	if !strings.Contains(s, "someone@example.com") {
		t.Errorf("description %q should mention the key comment", s)
	}
	if described == nil {
		t.Fatal("Describe returned a nil key for a key it found")
	}
	if described != Key(key) {
		t.Errorf("Describe returned %v, want the key it looked up", described)
	}
}

func TestDescribeUnknownKey(t *testing.T) {
	binding := KeyBindingImpl{KeyID: "missing-id"}

	s, described := binding.Describe(fakeFetcher{})

	if !strings.Contains(s, "Unknown key") {
		t.Errorf("description %q should say the key is unknown", s)
	}
	if !strings.Contains(s, "missing-id") {
		t.Errorf("description %q should name the missing ID", s)
	}
	if described != nil {
		t.Errorf("Describe returned %v for a key it could not find, want nil", described)
	}
}

func TestDescribeUsesBindingName(t *testing.T) {
	key := newTestSSHKey(t, "someone@example.com")
	lib := fakeFetcher{key.Id(): key}
	binding := KeyBindingImpl{KeyID: key.Id(), Name: "deploy"}

	s, _ := binding.Describe(lib)

	if !strings.HasPrefix(s, "deploy = ") {
		t.Errorf("description %q should be prefixed with the binding name", s)
	}
}

func TestGetSshLine(t *testing.T) {
	key := newTestSSHKey(t, "someone@example.com")
	lib := fakeFetcher{key.Id(): key}
	binding := KeyBindingImpl{KeyID: key.Id()}

	line, err := binding.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}

	if !strings.HasPrefix(line, "ssh-rsa ") {
		t.Errorf("authorized_keys line %q should start with the key type", line)
	}
	if !strings.HasSuffix(line, "someone@example.com") {
		t.Errorf("authorized_keys line %q should end with the comment", line)
	}
}

// A public key file with no trailing comment is perfectly legal, and the
// resulting key has an empty comment set. Rendering it must not panic.
func TestGetSshLineWithoutComment(t *testing.T) {
	key := newTestSSHKey(t, "")
	lib := fakeFetcher{key.Id(): key}
	binding := KeyBindingImpl{KeyID: key.Id()}

	line, err := binding.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}

	if !strings.HasPrefix(line, "ssh-rsa ") {
		t.Errorf("authorized_keys line %q should start with the key type", line)
	}
}

func TestGetSshLineMissingKey(t *testing.T) {
	binding := KeyBindingImpl{KeyID: "missing-id"}

	if _, err := binding.GetSshLine(fakeFetcher{}); err == nil {
		t.Error("GetSshLine should fail when the key is not in the library")
	}
}

func TestGetSshLineNonSSHKey(t *testing.T) {
	awsKey := NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "default")
	lib := fakeFetcher{awsKey.Id(): awsKey}
	binding := KeyBindingImpl{KeyID: awsKey.Id()}

	if _, err := binding.GetSshLine(lib); err == nil {
		t.Error("GetSshLine should fail for a key that is not an SSH key")
	}
}

func TestBindingLocations(t *testing.T) {
	// The persisted string values are part of the on-disk format.
	tests := []struct {
		loc  BindingLocation
		want string
	}{
		{FILE, "FILE"},
		{AUTHORIZED_KEYS, "AUTHORIZED_KEYS"},
		{AWS_CREDENTIALS, "CREDENTIALS"},
		{INSTANCE_ROOT_CREDENTIALS, "INSTANCE ROOT"},
	}

	for _, tc := range tests {
		if string(tc.loc) != tc.want {
			t.Errorf("binding location = %q, want %q", tc.loc, tc.want)
		}
	}
}

// A key discovered only as an AWS key-pair fingerprint has no public key
// material, so no authorized_keys line can be built for it. That is an error,
// not a crash.
func TestGetSshLineFingerprintOnlyKey(t *testing.T) {
	key := NewSSHKeyFromFingerprint("aws-keypair", time.Time{}, "ab:cd:ef")
	lib := fakeFetcher{key.Id(): key}
	binding := KeyBindingImpl{KeyID: key.Id()}

	if _, err := binding.GetSshLine(lib); err == nil {
		t.Error("GetSshLine should fail for a key with no public key material")
	}
}
