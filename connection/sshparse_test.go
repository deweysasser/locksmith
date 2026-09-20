package connection

import (
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
)

const authorizedKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDOzO+CaGCJLpMKrn2cqxb0Ln4L8SfKKG/qJfMNVUIFS8Xrqmw1DHHDLqSUvcvj1fxMtxwFI3JIrm4WHqCJCZ5aQKm0h4mOxnb0j/fZIXIUv8vp1bdCwJLKqcGF4/K1RQXJoTKPS0uLJrUW8rIX7YgZqZNbGfmKYBeCqAJfPMbHJDMIm5cYwRCAvwFmCn2N8w1UYs1U8lOgQyPFbzTLGO/VRbKcCFvCXLTKGvMQqVD3VwqKmxdmXKQKWO2PcSrLkFQeBqjX2fLxJzUDKGH0ZJ1mzQnDnU4y2hRbMZsWJYD8rPVJLMkNQbMZpVVZqGGXhWTYzFZnMRhNQQsRZaQV4z7B"

func TestParseAuthorizedKey(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey bool
	}{
		{"a plain key line", authorizedKey + " someone@example.com", true},
		{"a key with no comment", authorizedKey, true},
		{"a key with forced-command options", `command="/bin/true",no-pty ` + authorizedKey + " restricted", true},
		{"a blank line between entries", "", false},
		{"whitespace only", "   ", false},
		{"a comment line", "# my keys", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAuthorizedKey(tc.line, time.Now())

			if tc.wantKey && got == nil {
				t.Error("expected a key, got nil")
			}
			if !tc.wantKey && got != nil {
				t.Errorf("expected no key, got %v", got)
			}
		})
	}
}

// The comment on an authorized_keys line is what identifies the key to a human,
// so it has to survive parsing.
func TestParseAuthorizedKeyKeepsComment(t *testing.T) {
	key := parseAuthorizedKey(authorizedKey+" someone@example.com", time.Now())
	if key == nil {
		t.Fatal("expected a key")
	}

	sshKey, ok := key.(*data.SSHKey)
	if !ok {
		t.Fatalf("key is %T, want *data.SSHKey", key)
	}
	if !sshKey.Comments.Contains("someone@example.com") {
		t.Errorf("comments %v should hold the line comment", sshKey.Comments.StringArray())
	}
}

func TestSSHHostConnectionIdentity(t *testing.T) {
	c := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@host.example.com", Sudo: true}

	// The sudo flag is part of the rendered form, and therefore part of what
	// the substring filters match against.
	if got, want := c.String(), "ssh://ubuntu@host.example.com?sudo=true"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if c.Id() != data.IdFromString("ubuntu@host.example.com") {
		t.Errorf("Id() = %q, want the hash of the connection string", c.Id())
	}
}

func TestSSHHostConnectionIdDistinguishesHosts(t *testing.T) {
	a := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@a.example.com"}
	b := &SSHHostConnection{Type: "SSHHostConnection", Connection: "ubuntu@b.example.com"}

	if a.Id() == b.Id() {
		t.Error("different hosts should produce different connection IDs")
	}
}

func TestAWSConnectionIdentity(t *testing.T) {
	c := &AWSConnection{Type: "AWSConnection", Profile: "default"}

	if got, want := c.String(), "aws://default"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	// Unlike file and SSH connections, an AWS connection is identified by its
	// profile name verbatim rather than by a hash of it.
	if c.Id() != "default" {
		t.Errorf("Id() = %q, want the profile name", c.Id())
	}
}
