package command

import (
	"flag"
	"github.com/deweysasser/locksmith/data"
	"strings"
	"testing"

	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

// mkCtxWithArgs builds a context whose positional args are set (via flag.Parse).
func mkCtxWithArgs(args ...string) *cli.Context {
	set := flag.NewFlagSet("test", 0)
	_ = set.Parse(args)
	return cli.NewContext(nil, set, nil)
}

// mkCtxWithParentString builds a child context whose parent has a named string
// flag set to a given value (for GlobalString lookups).
func mkCtxWithParentString(name, value string) *cli.Context {
	parentSet := flag.NewFlagSet("parent", 0)
	parentSet.String(name, value, "")
	parent := cli.NewContext(nil, parentSet, nil)
	childSet := flag.NewFlagSet("child", 0)
	return cli.NewContext(nil, childSet, parent)
}

// mkCtx builds a cli.Context whose local flags have the given names defaulted
// to the given bool values. c.Bool(name) then returns that default.
func mkCtx(flags map[string]bool) *cli.Context {
	set := flag.NewFlagSet("test", 0)
	for k, v := range flags {
		set.Bool(k, v, "")
	}
	return cli.NewContext(nil, set, nil)
}

func TestAcceptAll(t *testing.T) {
	if !AcceptAll(nil) || !AcceptAll("anything") || !AcceptAll(42) {
		t.Error("AcceptAll should always be true")
	}
}

func TestBuildFilter_Empty(t *testing.T) {
	f := buildFilter(nil)
	if !f("anything") || !f(42) {
		t.Error("empty args should match everything")
	}
}

func TestBuildFilter_Substring(t *testing.T) {
	f := buildFilter([]string{"example"})
	if !f("me@login.example.com") {
		t.Error("should match substring")
	}
	if f("me@login.other.net") {
		t.Error("should reject non-matching")
	}
}

func TestBuildFilter_Union(t *testing.T) {
	f := buildFilter([]string{"aws", "example"})
	cases := map[string]bool{
		"aws:default":    true,
		"me@example.com": true,
		"root@other.net": false,
		"aws:example":    true, // matches both args; still true
	}
	for input, want := range cases {
		if got := f(input); got != want {
			t.Errorf("buildFilter union on %q: got %v, want %v", input, got, want)
		}
	}
}

func TestBuildFilter_UsesStringify(t *testing.T) {
	// Filter runs fmt.Sprintf("%s", i) on the candidate, so it works on
	// anything with a String() method — verify via a typed value.
	type named struct{ S string }
	f := buildFilter([]string{"hit"})
	if !f(named{S: "this is a hit"}) {
		t.Error("filter should stringify struct via fmt.Sprintf")
	}
}

func TestKeyAndAccountFilter_Wrap(t *testing.T) {
	// keyFilter and accountFilter just coerce a generic Filter to the typed
	// predicate; the logical behavior is buildFilter's. Verify pass-through
	// with a filter that accepts a known substring.
	base := buildFilter([]string{"abc"})
	kp := keyFilter(base)
	ap := accountFilter(base)
	// Both predicates take interface types — pass a string via a fmt.Stringer
	// proxy isn't possible since data.Key/data.Account are interfaces. We can
	// at minimum verify the wrapper isn't nil and forwards non-panic calls;
	// deeper integration is covered in the data and lib packages.
	if kp == nil || ap == nil {
		t.Fatal("wrappers returned nil")
	}
}

func TestOutputLevel_Debug(t *testing.T) {
	saved := output.Level
	defer func() { output.Level = saved }()
	output.Level = output.NormalLevel

	outputLevel(mkCtx(map[string]bool{"debug": true}))
	if output.Level != output.DebugLevel {
		t.Errorf("debug flag should set DebugLevel, got %v", output.Level)
	}
}

func TestOutputLevel_Verbose(t *testing.T) {
	saved := output.Level
	defer func() { output.Level = saved }()
	output.Level = output.NormalLevel

	outputLevel(mkCtx(map[string]bool{"verbose": true}))
	if output.Level != output.VerboseLevel {
		t.Errorf("verbose flag should set VerboseLevel, got %v", output.Level)
	}
}

func TestOutputLevel_NoFlag_LeavesLevelAlone(t *testing.T) {
	saved := output.Level
	defer func() { output.Level = saved }()
	output.Level = output.NormalLevel

	outputLevel(mkCtx(map[string]bool{"verbose": false, "debug": false}))
	if output.Level != output.NormalLevel {
		t.Errorf("no flag should leave level unchanged, got %v", output.Level)
	}
}

func TestBuildFilterFromContext(t *testing.T) {
	ctx := mkCtxWithArgs("aws", "example")
	f := buildFilterFromContext(ctx)
	if !f("me@example.com") {
		t.Error("filter from context should match substring")
	}
	if f("me@other.net") {
		t.Error("filter from context should reject non-match")
	}
}

func TestDatadir_Flag(t *testing.T) {
	// Clear env so it can't short-circuit the flag path.
	t.Setenv("LOCKSMITH_REPO", "")
	ctx := mkCtxWithParentString("repo", "/flag/path")
	if got := datadir(ctx); got != "/flag/path" {
		t.Errorf("datadir with --repo flag: got %q, want %q", got, "/flag/path")
	}
}

func TestDatadir_Env(t *testing.T) {
	t.Setenv("LOCKSMITH_REPO", "/env/path")
	set := flag.NewFlagSet("test", 0)
	ctx := cli.NewContext(nil, set, nil)
	if got := datadir(ctx); got != "/env/path" {
		t.Errorf("datadir with LOCKSMITH_REPO: got %q, want %q", got, "/env/path")
	}
}

func TestDatadir_Home(t *testing.T) {
	t.Setenv("LOCKSMITH_REPO", "")
	t.Setenv("HOME", "/home/testuser")
	set := flag.NewFlagSet("test", 0)
	ctx := cli.NewContext(nil, set, nil)
	if got, want := datadir(ctx), "/home/testuser/.x-locksmith"; got != want {
		t.Errorf("datadir from HOME: got %q, want %q", got, want)
	}
}

// --- filtering by identifier -----------------------------------------------
//
// The rendered form a command prints is lossy: StandardString truncates any ID
// over 25 characters to 22, and a SHA256 fingerprint is 50. Only the primary ID
// is rendered at all. So before this, a key could not be found by the
// fingerprint `ssh-keygen -l` prints, nor by its MD5 fingerprint, nor by any
// alias `add-id` had attached -- `locksmith list <fingerprint>` simply returned
// nothing, and so did `expire` and `remove` given the same argument.

// filterTestKey has a long primary ID and a secondary one, and renders itself
// the way StandardString does -- truncated.
type filterTestKey struct {
	ids []data.ID
}

func (k filterTestKey) Identifiers() []data.ID { return k.ids }

func (k filterTestKey) String() string {
	id := string(k.ids[0])
	if len(id) > 25 {
		id = id[:22] + "..."
	}
	return "key SSHKey " + id
}

const (
	filterTestSHA = "SHA256:sSu5Rney9ptujRrhY0A5V1RVohfaBXSfN55/ur1K5uc"
	filterTestMD5 = "e4:41:6c:b7:bf:14:91:7e:aa:1a:54:4d:4b:71:1b:fb"
)

func filterTestSubject() filterTestKey {
	return filterTestKey{ids: []data.ID{filterTestSHA, filterTestMD5}}
}

func TestFilterMatchesAFullFingerprintTheDisplayTruncates(t *testing.T) {
	key := filterTestSubject()

	// Precondition: the rendered form really does lose the fingerprint, or this
	// test would pass for the wrong reason.
	if strings.Contains(key.String(), filterTestSHA) {
		t.Fatalf("rendered form %q still contains the full fingerprint", key.String())
	}

	if !buildFilter([]string{filterTestSHA})(key) {
		t.Errorf("a key could not be found by its own primary fingerprint")
	}
}

func TestFilterMatchesASecondaryIdentifierThatIsNeverRendered(t *testing.T) {
	key := filterTestSubject()

	if strings.Contains(key.String(), filterTestMD5) {
		t.Fatalf("rendered form unexpectedly contains the MD5 fingerprint")
	}

	if !buildFilter([]string{filterTestMD5})(key) {
		t.Errorf("a key could not be found by its MD5 fingerprint, which is a stored ID")
	}
}

// `list` appends a key's disposition to the line before filtering, so
// `locksmith list remove` finds the keys on their way out. That suffix exists
// only in the rendered text, so matching identifiers must not replace it.
func TestFilterStillMatchesRenderedTextThatIsNotOnTheObject(t *testing.T) {
	key := filterTestSubject()
	line := key.String() + " [remove]"

	if !buildFilter([]string{"remove"})(rendered{text: line, object: key}) {
		t.Error("the disposition suffix stopped being filterable")
	}
	// ...and the object's identifiers are still reachable through the wrapper.
	if !buildFilter([]string{filterTestMD5})(rendered{text: line, object: key}) {
		t.Error("wrapping the object hid its identifiers from the filter")
	}
}

func TestFilterStillRejectsANonMatch(t *testing.T) {
	key := filterTestSubject()

	if buildFilter([]string{"SHA256:completely-different"})(key) {
		t.Error("filter matched a fingerprint the key does not have")
	}
	if buildFilter([]string{"nope"})(rendered{text: key.String(), object: key}) {
		t.Error("filter matched text that appears neither in the line nor in an ID")
	}
}

// An object with no identifiers must still filter on its rendered form alone.
func TestFilterWorksOnObjectsWithoutIdentifiers(t *testing.T) {
	if !buildFilter([]string{"example"})("me@example.com") {
		t.Error("a plain string stopped matching")
	}
	if buildFilter([]string{"example"})("me@other.net") {
		t.Error("a plain string matched when it should not")
	}
}

// filterTestSearchable has identifiers that do not contain the search term, so
// only the Searchable branch can match it.
type filterTestSearchable struct {
	filterTestKey
	terms []string
}

func (k filterTestSearchable) SearchTerms() []string { return k.terms }

// The form a user actually has in hand: the key material copied out of an
// authorized_keys line. It is in no identifier and in no rendered output, so
// without this branch pasting it returns nothing at all.
func TestFilterMatchesThePublicKeyBlob(t *testing.T) {
	const blob = "AAAAC3NzaC1lZDI1NTE5AAAAIBm6ZsMuHppbNCNDS0MX8DJWpYvdzsIWroeEd+4QTyPC"

	key := filterTestSearchable{
		filterTestKey: filterTestSubject(),
		terms:         []string{blob, "ssh-ed25519 " + blob},
	}

	// Precondition: nothing but SearchTerms can supply a match.
	if strings.Contains(key.String(), blob) {
		t.Fatal("the rendered form contains the blob; the test would pass for the wrong reason")
	}
	for _, id := range key.Identifiers() {
		if strings.Contains(string(id), blob) {
			t.Fatal("an identifier contains the blob; the test would pass for the wrong reason")
		}
	}

	if !buildFilter([]string{blob})(key) {
		t.Error("pasting the bare key blob found nothing")
	}
	if !buildFilter([]string{"ssh-ed25519 " + blob})(key) {
		t.Error("pasting a whole authorized_keys line found nothing")
	}
	if buildFilter([]string{"AAAAsomethingelse"})(key) {
		t.Error("filter matched a blob the key does not have")
	}
}
