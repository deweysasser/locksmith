package data

import (
	//	"encoding/base64"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/deweysasser/locksmith/output"
	"golang.org/x/crypto/ssh"
	"reflect"
	"strings"
	"time"
)

type PublicKey struct {
	Key ssh.PublicKey `json:",omitifempty"`
}

func (p *PublicKey) MarshalJSON() ([]byte, error) {
	if p.Key == nil {
		return json.Marshal(&struct {
			Type string
		}{
			Type: "UNKNOWN",
		})
	}

	return json.Marshal(&struct {
		Type, Data string
	}{
		Type: p.Key.Type(),
		Data: base64.StdEncoding.EncodeToString(p.Key.Marshal()),
	})
}

func (p *PublicKey) UnmarshalJSON(bytes []byte) error {
	temp := make(map[string]interface{})
	if e := json.Unmarshal(bytes, &temp); e != nil {
		return e
	}

	// Repository files are hand-mergeable JSON, so a malformed public key block
	// is an error to report rather than a reason to take the process down.
	keyType, ok := temp["Type"].(string)
	if !ok {
		return errors.New("public key has no Type field")
	}

	if "UNKNOWN" == keyType {
		return nil
	}

	encoded, ok := temp["Data"].(string)
	if !ok {
		return errors.New("public key has no Data field")
	}

	bKey, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return e
	}

	k, e := ssh.ParsePublicKey(bKey)
	if e != nil {
		return e
	}

	p.Key = k
	return nil
}

/** An SSH Key, public and (optionally) private
 */
type SSHKey struct {
	keyImpl
	Ids              IDList
	PublicKey        PublicKey
	Comments         StringSet
	haveIdsBeenAdded bool
}

func NewSSHKeyFromFingerprint(name string, tm time.Time, ids ...ID) *SSHKey {
	lIDs := IDList{}
	lIDs.AddArray(ids)

	names := StringSet{}
	names.Add(name)

	return &SSHKey{
		keyImpl{
			"SSHKey",
			names,
			false,
			"",
			tm,
		},
		lIDs,
		PublicKey{}, // this will have a nil underlying public key
		StringSet{},
		false,
	}
}

/* Use of a public Key, e.g. in an authorized_keys file
 */
type SSHBinding struct {
	Id      ID
	Comment string
	Options []string
}

func (s *SSHKey) Merge(k Key) {
	if other, ok := k.(*SSHKey); ok {
		s.keyImpl.Merge(&other.keyImpl)
		s.Comments.AddSet(other.Comments)
		s.Ids.AddList(&other.Ids)

	} else {
		panic("SSH asked to merge non-SSH key")
	}
}

func mergeIDArrays(a []ID, b []ID) []ID {
	r := make(map[ID]bool, 0)

	if a != nil {
		for _, id := range a {
			r[id] = true
		}
	}

	if b != nil {
		for _, id := range b {
			r[id] = true
		}
	}

	// Length, not capacity, would leave len(r) empty IDs in front of the real
	// ones.
	ra := make([]ID, 0, len(r))
	for k := range r {
		if k != "" {
			ra = append(ra, k)
		}
	}

	return ra
}

func NewSshKey(pub ssh.PublicKey, t time.Time) *SSHKey {
	return &SSHKey{
		keyImpl{
			"SSHKey",
			StringSet{},
			false, "",
			t,
		},
		IDList{},
		PublicKey{pub},
		StringSet{},
		false}
}

func (key *SSHKey) Id() ID {
	// A record with neither public key material nor a recorded fingerprint has
	// no identifiers at all.  That should not be fatal to whatever is listing
	// the library.
	ids := key.Identifiers()
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func (key *SSHKey) Identifiers() []ID {
	if !key.haveIdsBeenAdded {
		// We can only compute these if we have a public key
		if key.PublicKey.Key != nil {
			key.Ids.Add(ID(ssh.FingerprintSHA256(key.PublicKey.Key)))
			key.Ids.Add(ID(ssh.FingerprintLegacyMD5(key.PublicKey.Key)))
			// Complexity here:  we have an ssh.rsaPublicKey but need to turn it into an rsa.PublicKey

			// TODO:  This has been changed since the code was originally written.  Find the new way of doing this.
			//if cpker, ok := key.PublicKey.Key.(ssh.CryptoPublicKey); ok {
			//	if rsapk, ok := cpker.CryptoPublicKey().(*rsa.PublicKey); ok {
			//			if fp, err := ssh.FingerprintAWSPublic(rsapk); err == nil {
			//				key.Ids.Add(ID(fp))
			//			} else {
			//				output.Warn("Failed to compute AWS format fingerprint:", err)
			//			}
			//		}
			//	}
		}
		key.haveIdsBeenAdded = true
	}

	return key.Ids.Ids
}

func (key *SSHKey) String() string {
	return key.keyImpl.StandardString(key.Id(), key.Comments.StringArray()...)

	//return fmt.Sprintf("%s %s %s (%s)", key.Type, key.Id(), key.Comments.Join(", "), key.Names.Join(", "))
}

func (key *SSHKey) KeyType() string {
	return key.PublicKey.Key.Type()
}

func (key *SSHKey) Json() ([]byte, error) {
	return json.MarshalIndent(key, "", "  ")
}

func publicKey(keytype, pub string) ssh.PublicKey {
	line := fmt.Sprintf("%s %s", keytype, pub)
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	check(err)
	return pubkey
}

func (key *SSHKey) PublicKeyString() string {
	return string(ssh.MarshalAuthorizedKey(key.PublicKey.Key))
}

func getId(pub ssh.PublicKey) ID {
	return ID(ssh.FingerprintSHA256(pub))
}

func parseSshPrivateKey(content string, t time.Time, names ...string) Key {
	setNames := StringSet{}
	for _, s := range names {
		setNames.Add(s)
	}
	//if pk, err := ssh.ParseRawPrivateKey([]byte(content)) ; err == nil {
	if signer, err := ssh.ParsePrivateKey([]byte(content)); err == nil {
		pub := signer.PublicKey()
		/*
			var extras []ID
			if s, err := getAWSID(pk); err == nil {
				extras = append(extras, s)
			}
		*/
		return &SSHKey{
			keyImpl: keyImpl{
				Type:        "SSHKey",
				Names:       setNames,
				Deprecated:  false,
				Replacement: "",
				Earliest:    t},
			Ids:       IDList{},
			PublicKey: PublicKey{pub},
			Comments:  StringSet{},
		}
	}
	//}
	return nil
}

// openssl.exe pkcs8 -in ~/.ssh/AlignedWindowsInstancePair.pem -nocrypt -topk8 -outform DER | openssl sha1 -c
func getAWSID(iKey interface{}) (ID, error) {
	switch k := iKey.(type) {
	case *rsa.PrivateKey:
		output.Debug("Computing RSA AWS fingerprint")
		return ID(asHex(sha1.Sum(x509.MarshalPKCS1PrivateKey(k)))), nil
	case *ecdsa.PrivateKey:
		output.Debug("Computing ECDSA AWS fingerprint")
		if bytes, err := x509.MarshalECPrivateKey(k); err == nil {
			return ID(asHex(sha1.Sum(bytes))), nil
		}
	default:
		return ID(""), errors.New(fmt.Sprintf("Don't know how to compute AWS fingerprint for type %s", reflect.TypeOf(iKey)))
	}

	return ID(""), errors.New("Could not find key ID")
}

// asHex returns the given []byte as : separated string representation
func asHex(bytes [20]byte) string {
	var s []string
	for _, b := range bytes {
		s = append(s, fmt.Sprintf("%02x", b))
	}

	return strings.Join(s, ":")
}

// AuthorizedKeyOptions returns the option list preceding the key on an
// authorized_keys line, comma-separated and verbatim, or "" when the line
// carries none or cannot be parsed.
//
// x/crypto reports each option with its quoting intact, so joining on ","
// reproduces the original prefix byte for byte -- including commas inside a
// quoted value such as `from="10.0.0.0/8,!10.1.2.3"`.
func AuthorizedKeyOptions(line string) string {
	_, _, options, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return ""
	}
	return strings.Join(options, ",")
}

func parseSshPublicKey(content string, t time.Time, names []string) Key {
	//	pub, comment, options, _, err := ssh.ParseAuthorizedKey([]byte(content))
	if pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(content)); err == nil {

		comments := StringSet{}
		if comment != "" {
			comments.Add(comment)
		}

		sNames := StringSet{}
		for _, s := range names {
			if s != "" {
				sNames.Add(s)
			}
		}

		s := SSHKey{
			keyImpl: keyImpl{
				Type:        "SSHKey",
				Names:       sNames,
				Deprecated:  false,
				Replacement: "",
				Earliest:    t,
			},
			Ids:       IDList{},
			PublicKey: PublicKey{pub},
			Comments:  comments,
		}
		s.Identifiers() // Ensure the IDs are calculated

		return &s
	} else {
		output.Error("Failed to parse key", names)
	}
	return nil
}

func SSHLoadJson(s []byte) Key {
	key := new(SSHKey)
	json.Unmarshal(s, key)

	return key
}

// SearchTerms lets a user find a key by pasting the key material itself, which
// is what they have in front of them in an authorized_keys file or in the
// output of `ssh-keygen -y`.  It is not an identifier -- identifiers are
// fingerprints -- and it appears in no rendered form.
func (key *SSHKey) SearchTerms() []string {
	if key.PublicKey.Key == nil {
		return nil
	}

	blob := base64.StdEncoding.EncodeToString(key.PublicKey.Key.Marshal())

	// Both the bare blob and the whole line, because people paste both: the
	// blob when they have picked it out of a field, the full "type blob" when
	// they have copied a line wholesale.
	return []string{blob, key.PublicKey.Key.Type() + " " + blob}
}
