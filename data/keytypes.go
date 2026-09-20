package data

import (
	"strings"

	"golang.org/x/crypto/ssh"
)

// certSuffix is appended to a base algorithm name to form the OpenSSH
// certificate variant of it, e.g. "ssh-rsa-cert-v01@openssh.com".
const certSuffix = "-cert-v01@openssh.com"

// publicKeyAlgorithms are the algorithm names that can open an authorized_keys
// entry.  They are taken from x/crypto/ssh's own constants rather than written
// out here, so the set tracks the library instead of drifting from it.
//
// Recognising these by name matters: locksmith used to decide "this is a public
// key" by looking for the substring "ssh-" anywhere in the content, which
// happens to catch ssh-rsa, ssh-dss and ssh-ed25519 purely because of how they
// are spelled, and silently discards every ECDSA key ("ecdsa-sha2-nistp256")
// and every FIDO/security key ("sk-...").  A dropped key is invisible in a tool
// whose whole job is finding keys, so the check is now explicit.
var publicKeyAlgorithms = []string{
	ssh.KeyAlgoRSA,
	ssh.KeyAlgoDSA,
	ssh.KeyAlgoECDSA256,
	ssh.KeyAlgoECDSA384,
	ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoSKECDSA256,
	ssh.KeyAlgoED25519,
	ssh.KeyAlgoSKED25519,
	ssh.KeyAlgoRSASHA256,
	ssh.KeyAlgoRSASHA512,
}

// IsPublicKeyAlgorithm reports whether name is an SSH public key algorithm,
// either on its own or in its OpenSSH certificate form.
func IsPublicKeyAlgorithm(name string) bool {
	base := strings.TrimSuffix(name, certSuffix)

	for _, algo := range publicKeyAlgorithms {
		if base == algo {
			return true
		}
		// The security-key algorithms already end in "@openssh.com", so their
		// certificate form is spelled sk-ssh-ed25519-cert-v01@openssh.com --
		// the suffix replaces the vendor part rather than following it.
		if strings.HasSuffix(algo, "@openssh.com") &&
			base+"@openssh.com" == algo {
			return true
		}
	}

	return false
}

// looksLikeSSHPublicKey reports whether content holds something shaped like an
// authorized_keys entry: a line whose first field names a public key algorithm,
// or whose second does, when options precede the key.
//
// This is a shape check, not a parse.  It keeps NewKey from handing arbitrary
// files to the parser and logging an error for each one, while still letting
// anything that really is a key through.
func looksLikeSSHPublicKey(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// The algorithm is usually the first field, but options may precede it
		// and an option value can itself contain quoted whitespace --
		// command="/bin/ps -ef" splits into two fields on its own -- so the
		// algorithm can land at any index.  Scan the whole line.
		for _, field := range strings.Fields(line) {
			if IsPublicKeyAlgorithm(field) {
				return true
			}
		}
	}

	return false
}
