package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deweysasser/locksmith/connection"
)

func TestNewConnection_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	ctx := mkCtx(nil)
	got := NewConnection(path, ctx)
	fc, ok := got.(*connection.FileConnection)
	if !ok {
		t.Fatalf("want *FileConnection for existing path, got %T", got)
	}
	if fc.Path != path {
		t.Errorf("path = %q, want %q", fc.Path, path)
	}
}

func TestNewConnection_AWS(t *testing.T) {
	ctx := mkCtx(nil)
	got := NewConnection("aws:production", ctx)
	ac, ok := got.(*connection.AWSConnection)
	if !ok {
		t.Fatalf("want *AWSConnection for aws: prefix, got %T", got)
	}
	if ac.Profile != "production" {
		t.Errorf("profile = %q, want %q", ac.Profile, "production")
	}
}

func TestNewConnection_SSH(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		flags    map[string]bool
		wantSudo bool
	}{
		{"plain user no auto-sudo", "alice@host.example.com", nil, false},
		{"ubuntu auto-sudo", "ubuntu@host.example.com", nil, true},
		{"root auto-sudo", "root@host.example.com", nil, true},
		{"ec2-user auto-sudo", "ec2-user@host.example.com", nil, true},
		{"explicit sudo flag", "alice@host.example.com", map[string]bool{"sudo": true}, true},
		{"no-sudo overrides auto", "ubuntu@host.example.com", map[string]bool{"no-sudo": true}, false},
		{"no-sudo overrides explicit sudo", "alice@host.example.com", map[string]bool{"sudo": true, "no-sudo": true}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewConnection(tc.target, mkCtx(tc.flags))
			sc, ok := got.(*connection.SSHHostConnection)
			if !ok {
				t.Fatalf("want *SSHHostConnection, got %T", got)
			}
			if sc.Connection != tc.target {
				t.Errorf("connection = %q, want %q", sc.Connection, tc.target)
			}
			if sc.Sudo != tc.wantSudo {
				t.Errorf("sudo = %v, want %v", sc.Sudo, tc.wantSudo)
			}
		})
	}
}
