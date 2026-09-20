package data

import (
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// keytypeRead loads a fixture and hands it to NewKey, failing if nothing comes
// back. A nil return is the exact symptom of the dispatch bug these tests
// exist to prevent, so it is worth a clear message.
func keytypeRead(t *testing.T, fixture string) Key {
	t.Helper()

	body, err := ioutil.ReadFile("test-data/" + fixture)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", fixture, err)
	}

	key := NewKey(string(body), time.Now(), fixture)
	if key == nil {
		t.Fatalf("NewKey dropped %s -- locksmith would never see this key", fixture)
	}
	return key
}

// Every algorithm locksmith claims to catalog has to survive NewKey. ECDSA is
// the one that used to fall through: the old dispatch looked for the substring
// "ssh-" anywhere in the content, which ecdsa-sha2-nistp256 does not contain.
func TestNewKeyAcceptsEveryAlgorithm(t *testing.T) {
	tests := []struct {
		fixture string
		keyType string
	}{
		{"rsa.pub", ssh.KeyAlgoRSA},
		{"ed25519.pub", ssh.KeyAlgoED25519},
		{"ecdsa256.pub", ssh.KeyAlgoECDSA256},
		{"ecdsa384.pub", ssh.KeyAlgoECDSA384},
		{"ecdsa521.pub", ssh.KeyAlgoECDSA521},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			key := keytypeRead(t, tc.fixture)

			sshKey, ok := key.(*SSHKey)
			if !ok {
				t.Fatalf("got %T, want *SSHKey", key)
			}
			if got := sshKey.KeyType(); got != tc.keyType {
				t.Errorf("KeyType() = %q, want %q", got, tc.keyType)
			}
			if sshKey.Id() == "" {
				t.Error("key has no identifier")
			}
		})
	}
}

// The public and private halves of an ECDSA pair must agree on identity, the
// same property that lets locksmith match a local key to a remote one.
func TestECDSAHalvesShareAnId(t *testing.T) {
	for _, bits := range []string{"256", "384", "521"} {
		t.Run(bits, func(t *testing.T) {
			pub := keytypeRead(t, "ecdsa"+bits+".pub")
			priv := keytypeRead(t, "ecdsa"+bits)

			if pub.Id() != priv.Id() {
				t.Errorf("public ID %q != private ID %q", pub.Id(), priv.Id())
			}
		})
	}
}

func TestECDSAFingerprintsMatchSSHLibrary(t *testing.T) {
	key := keytypeRead(t, "ecdsa256.pub").(*SSHKey)

	want := ID(ssh.FingerprintSHA256(key.PublicKey.Key))
	if key.Id() != want {
		t.Errorf("Id() = %q, want %q", key.Id(), want)
	}

	ids := key.Identifiers()
	if len(ids) != 2 {
		t.Fatalf("Identifiers() = %v, want SHA256 and legacy MD5", ids)
	}
	if ids[1] != ID(ssh.FingerprintLegacyMD5(key.PublicKey.Key)) {
		t.Errorf("second identifier = %q, want the legacy MD5 fingerprint", ids[1])
	}
}

func TestECDSAWithoutComment(t *testing.T) {
	key := keytypeRead(t, "ecdsa-nocomment.pub")

	if key.Id() == "" {
		t.Error("a key with no comment should still have an identifier")
	}
}

// Keys of different algorithms must never collide on identity.
func TestKeyIdsAreDistinctAcrossAlgorithms(t *testing.T) {
	fixtures := []string{"rsa.pub", "ed25519.pub", "ecdsa256.pub", "ecdsa384.pub", "ecdsa521.pub"}

	seen := make(map[ID]string)
	for _, f := range fixtures {
		id := keytypeRead(t, f).Id()
		if prev, dup := seen[id]; dup {
			t.Errorf("%s and %s share identifier %q", prev, f, id)
		}
		seen[id] = f
	}
}

func TestIsPublicKeyAlgorithm(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{ssh.KeyAlgoRSA, true},
		{ssh.KeyAlgoDSA, true},
		{ssh.KeyAlgoED25519, true},
		{ssh.KeyAlgoECDSA256, true},
		{ssh.KeyAlgoECDSA384, true},
		{ssh.KeyAlgoECDSA521, true},
		{ssh.KeyAlgoSKECDSA256, true},
		{ssh.KeyAlgoSKED25519, true},
		{ssh.KeyAlgoRSASHA256, true},
		{ssh.KeyAlgoRSASHA512, true},
		// Certificate forms name their base algorithm.
		{"ssh-rsa-cert-v01@openssh.com", true},
		{"ssh-ed25519-cert-v01@openssh.com", true},
		{"ecdsa-sha2-nistp256-cert-v01@openssh.com", true},
		{"sk-ssh-ed25519-cert-v01@openssh.com", true},
		// Not algorithms.
		{"", false},
		{"ssh", false},
		{"ssh-", false},
		{"ssh-rsa-but-not-really", false},
		{"AAAAB3NzaC1yc2EAAAADAQAB", false},
		{"command=\"/bin/true\"", false},
	}

	for _, tc := range tests {
		if got := IsPublicKeyAlgorithm(tc.name); got != tc.want {
			t.Errorf("IsPublicKeyAlgorithm(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLooksLikeSSHPublicKey(t *testing.T) {
	const ecdsaLine = "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY= someone@example.com"

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"an ecdsa line", ecdsaLine, true},
		{"an ed25519 line", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 someone@example.com", true},
		{"an rsa line", "ssh-rsa AAAAB3NzaC1yc2E someone@example.com", true},
		{"options before the key", `no-pty,no-X11-forwarding ` + ecdsaLine, true},
		// An option value can hold quoted whitespace, which splits into extra
		// fields and pushes the algorithm further along the line.
		{"a forced command containing a space", `command="/bin/ps -ef",no-pty ` + ecdsaLine, true},
		{"leading blank lines and comments", "\n# my keys\n\n" + ecdsaLine, true},
		{"empty", "", false},
		{"prose", "this file is not a key at all", false},
		{"a comment mentioning a key", "# remember to rotate the ssh-rsa key", false},
		{"a PEM private key body", "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXkt\n", false},
		{"bare base64", "AAAAB3NzaC1yc2EAAAADAQABAAABAQ==", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeSSHPublicKey(tc.content); got != tc.want {
				t.Errorf("looksLikeSSHPublicKey(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

// A private key file must route to the private key parser even though its
// decoded body names an algorithm.
func TestNewKeyPrefersPrivateKeyParsing(t *testing.T) {
	for _, fixture := range []string{"ecdsa256", "ed25519", "rsa"} {
		t.Run(fixture, func(t *testing.T) {
			key := keytypeRead(t, fixture)

			sshKey, ok := key.(*SSHKey)
			if !ok {
				t.Fatalf("got %T, want *SSHKey", key)
			}
			if sshKey.PublicKey.Key == nil {
				t.Error("a private key should yield its public half")
			}
		})
	}
}

// The ECDSA entry appended to the fixture authorized_keys file has to come back
// out, which is the end-to-end version of the dispatch fix.
func TestAuthorizedKeysFileYieldsEcdsaEntry(t *testing.T) {
	body, err := ioutil.ReadFile("test-data/authorized_keys")
	if err != nil {
		t.Fatalf("reading authorized_keys: %v", err)
	}

	var found bool
	for _, line := range splitLines(string(body)) {
		key := NewKey(line, time.Now())
		if key == nil {
			continue
		}
		if sshKey, ok := key.(*SSHKey); ok && sshKey.KeyType() == ssh.KeyAlgoECDSA256 {
			found = true
		}
	}

	if !found {
		t.Error("the ecdsa-sha2-nistp256 entry in authorized_keys was not parsed")
	}
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
