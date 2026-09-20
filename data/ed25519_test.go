package data

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// Ed25519 has been parseable by x/crypto/ssh for years, and locksmith's
// dispatch in NewKey catches it by accident rather than by design. These tests
// pin the behaviour down so a future change to the dispatch, the fingerprint
// set or the JSON shape cannot quietly drop support for it.

const (
	edtestPublicPath     = "test-data/ed25519.pub"
	edtestPrivatePath    = "test-data/ed25519"
	edtestNoCommentPath  = "test-data/ed25519-nocomment.pub"
	edtestEncryptedPath  = "test-data/ed25519-encrypted"
	edtestComment        = "dewey@locksmith-test"
	edtestKeyType        = "ssh-ed25519"
	edtestRSAPublicPath  = "test-data/rsa.pub"
	edtestDemoPublicPath = "test-data/public-keys/ed25519-demo.pub"
)

// edtestRead reads a fixture and asserts it came back as an SSH key.
func edtestRead(t *testing.T, path string) *SSHKey {
	t.Helper()

	key := Read(path)
	if key == nil {
		t.Fatalf("Read(%q) returned nil, want a key", path)
	}

	sshKey, ok := key.(*SSHKey)
	if !ok {
		t.Fatalf("Read(%q) returned %T, want *SSHKey", path, key)
	}

	return sshKey
}

func TestEd25519PublicKeyParse(t *testing.T) {
	key := edtestRead(t, edtestPublicPath)

	if got := key.KeyType(); got != edtestKeyType {
		t.Errorf("KeyType() = %q, want %q", got, edtestKeyType)
	}
	if !key.Comments.Contains(edtestComment) {
		t.Errorf("comments = %v, want %q", key.Comments.StringArray(), edtestComment)
	}
	if got := key.PublicKeyString(); !strings.HasPrefix(got, edtestKeyType+" ") {
		t.Errorf("PublicKeyString() = %q, want it to start with %q", got, edtestKeyType)
	}
}

func TestEd25519PrivateKeyParse(t *testing.T) {
	key := edtestRead(t, edtestPrivatePath)

	if got := key.KeyType(); got != edtestKeyType {
		t.Errorf("KeyType() = %q, want %q", got, edtestKeyType)
	}
	if key.PublicKey.Key == nil {
		t.Fatal("parsing a private key should recover its public half")
	}
}

// Correlating a private key on a laptop with an authorized_keys entry on a
// server is the whole point of the fingerprint ID, so both halves of one
// keypair must produce the same identifiers.
func TestEd25519PublicAndPrivateHalvesShareAnId(t *testing.T) {
	pub := edtestRead(t, edtestPublicPath)
	priv := edtestRead(t, edtestPrivatePath)

	if pub.Id() != priv.Id() {
		t.Errorf("public half ID = %q, private half ID = %q, want them equal", pub.Id(), priv.Id())
	}
	if got, want := priv.PublicKeyString(), pub.PublicKeyString(); got != want {
		t.Errorf("private half public key = %q, want %q", got, want)
	}

	for _, id := range pub.Identifiers() {
		if !priv.Ids.Contains(id) {
			t.Errorf("private half is missing identifier %q", id)
		}
	}
}

func TestEd25519Identifiers(t *testing.T) {
	key := edtestRead(t, edtestPublicPath)

	ids := key.Identifiers()
	if len(ids) != 2 {
		t.Fatalf("Identifiers() = %v, want the SHA256 and legacy MD5 fingerprints", ids)
	}
	if !strings.HasPrefix(string(ids[0]), "SHA256:") {
		t.Errorf("first identifier = %q, want the SHA256 fingerprint first", ids[0])
	}
	if strings.HasPrefix(string(ids[1]), "SHA256:") || !strings.Contains(string(ids[1]), ":") {
		t.Errorf("second identifier = %q, want the legacy MD5 fingerprint", ids[1])
	}
	if key.Id() != ids[0] {
		t.Errorf("Id() = %q, want the first identifier %q", key.Id(), ids[0])
	}

	// Identifiers() computes lazily and caches; repeated calls must not
	// accumulate duplicates or reorder the list.
	again := key.Identifiers()
	if len(again) != len(ids) {
		t.Fatalf("second call to Identifiers() = %v, want %v", again, ids)
	}
	for i := range ids {
		if again[i] != ids[i] {
			t.Errorf("identifier[%d] = %q on the second call, want %q", i, again[i], ids[i])
		}
	}
}

func TestEd25519FingerprintsMatchSSHLibrary(t *testing.T) {
	key := edtestRead(t, edtestPublicPath)

	want := ID(ssh.FingerprintSHA256(key.PublicKey.Key))
	if key.Id() != want {
		t.Errorf("Id() = %q, want %q", key.Id(), want)
	}
	if legacy := ID(ssh.FingerprintLegacyMD5(key.PublicKey.Key)); !key.Ids.Contains(legacy) {
		t.Errorf("identifiers %v are missing the legacy MD5 fingerprint %q", key.Ids.Ids, legacy)
	}
}

func TestEd25519AuthorizedKeysLine(t *testing.T) {
	pub := edtestRead(t, edtestPublicPath)
	line := strings.TrimSpace(pub.PublicKeyString())

	tests := []struct {
		name        string
		content     string
		wantComment string
	}{
		{"bare key", line, ""},
		{"key with a comment", line + " " + edtestComment, edtestComment},
		{
			"key with options",
			`command="/bin/ps -ef",no-port-forwarding,no-pty ` + line + " " + edtestComment,
			edtestComment,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k := NewKey(tc.content, time.Time{}, "authorized_keys")
			if k == nil {
				t.Fatalf("NewKey(%q) returned nil", tc.content)
			}

			sshKey, ok := k.(*SSHKey)
			if !ok {
				t.Fatalf("NewKey returned %T, want *SSHKey", k)
			}
			if got := sshKey.KeyType(); got != edtestKeyType {
				t.Errorf("KeyType() = %q, want %q", got, edtestKeyType)
			}
			if sshKey.Id() != pub.Id() {
				t.Errorf("Id() = %q, want %q", sshKey.Id(), pub.Id())
			}
			if tc.wantComment == "" {
				if got := sshKey.Comments.StringArray(); len(got) != 0 {
					t.Errorf("comments = %v, want none", got)
				}
			} else if !sshKey.Comments.Contains(tc.wantComment) {
				t.Errorf("comments = %v, want %q", sshKey.Comments.StringArray(), tc.wantComment)
			}
			if !sshKey.Names.Contains("authorized_keys") {
				t.Errorf("names = %v, want the name it was read under", sshKey.Names.StringArray())
			}
		})
	}
}

// A public key file with no trailing comment is legal, and is what ssh-keygen
// produces when it is given an empty comment string.
func TestEd25519PublicKeyWithoutComment(t *testing.T) {
	key := edtestRead(t, edtestNoCommentPath)

	if got := key.KeyType(); got != edtestKeyType {
		t.Errorf("KeyType() = %q, want %q", got, edtestKeyType)
	}
	if got := key.Comments.StringArray(); len(got) != 0 {
		t.Errorf("comments = %v, want none", got)
	}
	if key.Id() == "" {
		t.Error("a key with no comment should still have an ID")
	}
}

func TestEd25519JsonRoundTrip(t *testing.T) {
	key := edtestRead(t, edtestPublicPath)
	key.Names.Add("ed25519.pub")

	bytes, err := key.Json()
	if err != nil {
		t.Fatalf("Json: %v", err)
	}

	restored, ok := SSHLoadJson(bytes).(*SSHKey)
	if !ok {
		t.Fatal("SSHLoadJson did not return an *SSHKey")
	}

	if restored.Id() != key.Id() {
		t.Errorf("restored ID = %q, want %q", restored.Id(), key.Id())
	}
	if got := restored.KeyType(); got != edtestKeyType {
		t.Errorf("restored KeyType() = %q, want %q", got, edtestKeyType)
	}
	if !restored.Comments.Contains(edtestComment) {
		t.Errorf("restored comments = %v, want %q", restored.Comments.StringArray(), edtestComment)
	}
	if !restored.Names.Contains("ed25519.pub") {
		t.Errorf("restored names = %v, want the name to survive", restored.Names.StringArray())
	}
	if got, want := restored.PublicKeyString(), key.PublicKeyString(); got != want {
		t.Errorf("restored public key = %q, want %q", got, want)
	}
}

// Two sightings of the same Ed25519 key -- say one in a local file and one in a
// remote authorized_keys -- have to collapse into a single record.
func TestEd25519Merge(t *testing.T) {
	early := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

	line := strings.TrimSpace(edtestRead(t, edtestPublicPath).PublicKeyString())

	first, ok := NewKey(line+" "+edtestComment, late, "ed25519.pub").(*SSHKey)
	if !ok {
		t.Fatal("failed to parse the first sighting")
	}
	second, ok := NewKey(line+" someone-else@example.com", early, "authorized_keys").(*SSHKey)
	if !ok {
		t.Fatal("failed to parse the second sighting")
	}

	if first.Id() != second.Id() {
		t.Fatalf("the two sightings disagree on the ID: %q vs %q", first.Id(), second.Id())
	}

	first.Merge(second)

	if !first.Comments.Contains(edtestComment) || !first.Comments.Contains("someone-else@example.com") {
		t.Errorf("merged comments = %v, want both", first.Comments.StringArray())
	}
	if !first.Names.Contains("ed25519.pub") || !first.Names.Contains("authorized_keys") {
		t.Errorf("merged names = %v, want both", first.Names.StringArray())
	}
	if !first.Earliest.Equal(early) {
		t.Errorf("merged Earliest = %s, want the earlier sighting %s", first.Earliest, early)
	}
	if len(first.Identifiers()) != 2 {
		t.Errorf("merged identifiers = %v, want no duplicates", first.Identifiers())
	}
}

// The authorized_keys line locksmith writes when applying a change has to be
// something sshd -- and therefore x/crypto/ssh -- will accept back.
func TestEd25519GetSshLineRoundTrips(t *testing.T) {
	key := edtestRead(t, edtestPublicPath)
	lib := fakeFetcher{key.Id(): key}
	binding := KeyBindingImpl{KeyID: key.Id(), Location: AUTHORIZED_KEYS}

	line, err := binding.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}

	if !strings.HasPrefix(line, edtestKeyType+" ") {
		t.Errorf("authorized_keys line %q should start with %q", line, edtestKeyType)
	}
	if !strings.HasSuffix(line, edtestComment) {
		t.Errorf("authorized_keys line %q should end with the comment", line)
	}

	parsed, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		t.Fatalf("ssh.ParseAuthorizedKey(%q): %v", line, err)
	}
	if parsed.Type() != edtestKeyType {
		t.Errorf("re-parsed key type = %q, want %q", parsed.Type(), edtestKeyType)
	}
	if comment != edtestComment {
		t.Errorf("re-parsed comment = %q, want %q", comment, edtestComment)
	}
	if got := ID(ssh.FingerprintSHA256(parsed)); got != key.Id() {
		t.Errorf("re-parsed fingerprint = %q, want %q", got, key.Id())
	}
}

func TestEd25519GetSshLineWithoutComment(t *testing.T) {
	key := edtestRead(t, edtestNoCommentPath)
	lib := fakeFetcher{key.Id(): key}
	binding := KeyBindingImpl{KeyID: key.Id()}

	line, err := binding.GetSshLine(lib)
	if err != nil {
		t.Fatalf("GetSshLine: %v", err)
	}

	parsed, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		t.Fatalf("ssh.ParseAuthorizedKey(%q): %v", line, err)
	}
	if comment != "" {
		t.Errorf("re-parsed comment = %q, want none", comment)
	}
	if got := ID(ssh.FingerprintSHA256(parsed)); got != key.Id() {
		t.Errorf("re-parsed fingerprint = %q, want %q", got, key.Id())
	}
}

// Distinct keys -- of the same type or of different types -- must never share
// an identifier, or the library would merge unrelated records.
func TestEd25519IdsDoNotCollide(t *testing.T) {
	ed := edtestRead(t, edtestPublicPath)
	other := edtestRead(t, edtestNoCommentPath)
	demo := edtestRead(t, edtestDemoPublicPath)
	rsa := edtestRead(t, edtestRSAPublicPath)

	seen := make(map[ID]string)
	for _, k := range []struct {
		name string
		key  *SSHKey
	}{
		{"ed25519.pub", ed},
		{"ed25519-nocomment.pub", other},
		{"public-keys/ed25519-demo.pub", demo},
		{"rsa.pub", rsa},
	} {
		for _, id := range k.key.Identifiers() {
			if prev, ok := seen[id]; ok {
				t.Errorf("%s and %s share identifier %q", prev, k.name, id)
			}
			seen[id] = k.name
		}
	}
}

func TestEd25519StringIsReadable(t *testing.T) {
	key := edtestRead(t, edtestPublicPath)

	s := key.String()
	if !strings.Contains(s, "SSHKey") {
		t.Errorf("String() = %q, want it to name the type", s)
	}
	if !strings.Contains(s, edtestComment) {
		t.Errorf("String() = %q, want it to carry the comment", s)
	}
	// The SHA256 fingerprint is longer than the column StandardString
	// reserves, so it is truncated rather than blowing the layout apart.
	if !strings.Contains(s, string(key.Id())[:22]) {
		t.Errorf("String() = %q, want it to show the leading part of %q", s, key.Id())
	}
}

// Passphrase-protected private keys are out of scope -- locksmith has no way to
// ask for the passphrase. What matters is that they are declined cleanly rather
// than taking the fetch down.
func TestEd25519EncryptedPrivateKeyIsDeclined(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("reading a passphrase-protected key panicked: %v", r)
		}
	}()

	if key := Read(edtestEncryptedPath); key != nil {
		t.Errorf("Read(%q) = %v, want nil for a key we cannot decrypt", edtestEncryptedPath, key)
	}
}
