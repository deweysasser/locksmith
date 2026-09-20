package command

import (
	"testing"
	"time"

	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
)

func keyLibraryWith(t *testing.T, keys ...data.Key) lib.KeyLibrary {
	t.Helper()

	l := lib.NewKeyLibrary(t.TempDir())
	for _, k := range keys {
		if err := l.Store(k); err != nil {
			t.Fatalf("storing key: %v", err)
		}
	}
	return l
}

// Adding an ID by hand has to land on exactly one key, so an ambiguous filter
// is refused rather than guessed at.
func TestFindKey(t *testing.T) {
	silence(t)

	sshKey := newSSHTestKey(t)

	tests := []struct {
		name    string
		keys    []data.Key
		filter  []string
		wantErr bool
	}{
		{
			name:    "no keys at all",
			keys:    nil,
			filter:  []string{"anything"},
			wantErr: true,
		},
		{
			name:    "nothing matches",
			keys:    []data.Key{sshKey},
			filter:  []string{"no-such-key"},
			wantErr: true,
		},
		{
			// Filters match the rendered form of the key, in which a long
			// fingerprint is truncated -- so match on the comment instead.
			name:    "exactly one match",
			keys:    []data.Key{sshKey},
			filter:  []string{"someone@example.com"},
			wantErr: false,
		},
		{
			name:    "an AWS key cannot take extra IDs",
			keys:    []data.Key{data.NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")},
			filter:  []string{"AKIAEXAMPLE"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := keyLibraryWith(t, tc.keys...)

			got, err := findKey(l, buildFilter(tc.filter))

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected an error, got key %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Error("expected a key, got nil")
			}
		})
	}
}

func TestFindKeyRefusesAmbiguousMatches(t *testing.T) {
	silence(t)

	l := keyLibraryWith(t,
		data.NewAwsKey("AKIAONE", time.Time{}, true, "shared-name"),
		data.NewAwsKey("AKIATWO", time.Time{}, true, "shared-name"),
	)

	if _, err := findKey(l, buildFilter([]string{"shared-name"})); err == nil {
		t.Error("a filter matching two keys should be refused")
	}
}

// getKeyIds is what `add` uses to decide which keys to bind; a key already on
// its way out should never be handed to a new account.
func TestGetKeyIdsSkipsDeprecatedKeys(t *testing.T) {
	silence(t)

	live := data.NewAwsKey("AKIALIVE", time.Time{}, true, "prod")
	dead := data.NewAwsKey("AKIADEAD", time.Time{}, true, "prod")
	dead.Expire()

	l := keyLibraryWith(t, live, dead)

	got := getKeyIds(l, lib.NewPolicyLibrary(t.TempDir()), keyFilter(buildFilter([]string{"prod"})))

	if len(got) != 1 || got[0] != "AKIALIVE" {
		t.Errorf("got %v, want only the live key", got)
	}
}

func TestGetKeyIdsWithNoMatches(t *testing.T) {
	silence(t)

	l := keyLibraryWith(t, data.NewAwsKey("AKIALIVE", time.Time{}, true, "prod"))

	if got := getKeyIds(l, lib.NewPolicyLibrary(t.TempDir()), keyFilter(buildFilter([]string{"nothing"}))); len(got) != 0 {
		t.Errorf("got %v, want nothing", got)
	}
}

// changestr renders a change as its account when the account is known, and
// falls back to the change itself when it is not.
func TestChangestr(t *testing.T) {
	silence(t)

	ml := lib.MainLibrary{Path: t.TempDir()}
	acct := data.NewSSHAccount("root", "host.example.com", "conn1", nil)
	if err := ml.Accounts().Store(acct); err != nil {
		t.Fatalf("storing account: %v", err)
	}

	known := changestr(ml.Accounts(), data.Change{Type: "Change", Account: acct.Id()})
	if _, ok := known.(data.Account); !ok {
		t.Errorf("changestr returned %T for a known account, want a data.Account", known)
	}

	unknown := changestr(ml.Accounts(), data.Change{Type: "Change", Account: "root@gone.example.com"})
	if _, ok := unknown.(data.Change); !ok {
		t.Errorf("changestr returned %T for an unknown account, want the change itself", unknown)
	}
}

const testPublicKeyLine = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDOzO+CaGCJLpMKrn2cqxb0Ln4L8SfKKG/qJfMNVUIFS8Xrqmw1DHHDLqSUvcvj1fxMtxwFI3JIrm4WHqCJCZ5aQKm0h4mOxnb0j/fZIXIUv8vp1bdCwJLKqcGF4/K1RQXJoTKPS0uLJrUW8rIX7YgZqZNbGfmKYBeCqAJfPMbHJDMIm5cYwRCAvwFmCn2N8w1UYs1U8lOgQyPFbzTLGO/VRbKcCFvCXLTKGvMQqVD3VwqKmxdmXKQKWO2PcSrLkFQeBqjX2fLxJzUDKGH0ZJ1mzQnDnU4y2hRbMZsWJYD8rPVJLMkNQbMZpVVZqGGXhWTYzFZnMRhNQQsRZaQV4z7B someone@example.com"

func newSSHTestKey(t *testing.T) *data.SSHKey {
	t.Helper()

	k := data.NewKey(testPublicKeyLine, time.Now())
	if k == nil {
		t.Fatal("failed to parse the test public key")
	}
	sshKey, ok := k.(*data.SSHKey)
	if !ok {
		t.Fatalf("parsed key is %T, want *data.SSHKey", k)
	}
	return sshKey
}
