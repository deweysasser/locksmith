package command

import (
	"flag"
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
