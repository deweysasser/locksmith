# Test fixtures

Everything here is throwaway material generated for the test suite. **None of
it is live.** The private keys are real, usable, passphrase-less keys — they are
committed deliberately, they authorize nothing, and they should never be treated
as an example to follow for real keys.

This repository is public, so fixtures carry neutral names and comments. If you
add one, generate it fresh and comment it `something@locksmith-test`; do not
copy material out of a real machine, a real account or a real fleet. Newer tests
generate keys in-process instead (see `dotestPublicKey` in
`connection/doconnection_test.go`), which is the better pattern for anything new.

| Fixture | What it is for |
|---|---|
| `rsa`, `rsa.pub` | RSA keypair; the public/private halves must agree on identity |
| `dss`, `dss.pub` | DSA keypair, for the legacy algorithm path |
| `ecdsa256/384/521{,.pub}` | ECDSA at all three curve sizes — the algorithms the old `"ssh-"` substring dispatch silently dropped |
| `ecdsa-nocomment.pub` | a key with no trailing comment |
| `ed25519{,.pub}` | Ed25519 keypair |
| `ed25519-nocomment.pub` | Ed25519 with no comment |
| `ed25519-encrypted{,.pub}` | passphrase-protected; must be declined cleanly, not parsed and not panicked on |
| `constrained{,.pub}` | key whose `authorized_keys` line carries options |
| `authorized_keys` | several entries, including a forced-command line whose quoted option value contains a space, and one entry per algorithm |
| `credentials` | AWS shared-credentials format. The key IDs are AWS's own documentation placeholders (`AKIAIOSFODNN7EXAMPLE`); the secrets are obviously fake |
| `locksmith-test-aws-generated.pem` | AWS-style PEM, for the key-pair fingerprint path |
| `public-keys/` | a directory swept as a whole, to exercise recursive ingestion across a mix of algorithms and comment shapes |
