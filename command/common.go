package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"os"
	"strings"
)

// Return the locksmith data directory
func datadir(c *cli.Context) string {
	if s := c.GlobalString("repo"); s != "" {
		output.Debug("Repo from --repo flag:", s)
		return s
	}
	if repo := os.Getenv("LOCKSMITH_REPO"); repo != "" {
		output.Debug("Repo from env:", repo)
		return repo
	}

	var r string
	if home := os.Getenv("HOME"); home != "" {
		r = home + "/.x-locksmith"
	} else {
		if profile := os.Getenv("USERPROFILE"); profile != "" {
			r = profile + "/locksmith"
		}
	}
	output.Debug("Repo in home directory:", r)
	return r
}

type Filter func(interface{}) bool

func buildFilterFromContext(c *cli.Context) Filter {
	return buildFilter(c.Args())
}

func AcceptAll(a interface{}) bool {
	return true
}

// rendered pairs an object with the text a command prints for it, so a filter
// can match either.  `list` needs this: it appends a key's disposition to the
// line before filtering, so that `locksmith list remove` finds exactly the keys
// on their way out, and that suffix exists nowhere on the object.
type rendered struct {
	text   string
	object interface{}
}

// unwrap splits a filter candidate into the text to match and the object to
// interrogate.  Most call sites pass the object itself; `list` and `plan` pass
// a string they have already assembled.
func unwrap(i interface{}) (string, interface{}) {
	if r, ok := i.(rendered); ok {
		return r.text, r.object
	}
	return fmt.Sprintf("%s", i), i
}

func buildFilter(args []string) Filter {
	if len(args) == 0 {
		return AcceptAll
	}

	return func(i interface{}) bool {
		text, candidate := unwrap(i)

		for _, s := range args {
			if strings.Contains(text, s) {
				return true
			}
		}

		// Identifiers are matched as well as the rendered text, because the
		// rendered text is lossy in two ways that made a key unfindable by the
		// fingerprint `ssh-keygen -l` prints.  StandardString truncates any ID
		// over 25 characters to 22, and a SHA256 fingerprint is 50; and only
		// the *primary* ID is ever rendered, so a key's MD5 fingerprint and any
		// alias added by `add-id` appeared nowhere at all.
		if ids, ok := candidate.(data.Identiferser); ok {
			for _, id := range ids.Identifiers() {
				for _, s := range args {
					if strings.Contains(string(id), s) {
						return true
					}
				}
			}
		}

		// ...and by whatever else identifies the object to a human.  For an SSH
		// key that is the base64 blob itself, which is what someone has in hand
		// when they copy a line out of authorized_keys -- neither a fingerprint
		// nor anything the display shows.
		if terms, ok := candidate.(data.Searchable); ok {
			for _, term := range terms.SearchTerms() {
				for _, s := range args {
					if strings.Contains(term, s) {
						return true
					}
				}
			}
		}

		return false
	}
}

func accountFilter(filter Filter) lib.AccountPredicate {
	return func(account data.Account) bool {
		return filter(account)
	}
}

func keyFilter(filter Filter) lib.KeyPredicate {
	return func(key data.Key) bool {
		output.Debug("Checking", key)
		return filter(key)
	}
}

func outputLevel(c *cli.Context) {
	switch {
	case c.Bool("debug") || c.GlobalBool("debug"):
		output.Level = output.DebugLevel
	case c.Bool("verbose") || c.GlobalBool("verbose"):
		output.Level = output.VerboseLevel
	case c.Bool("silent") || c.GlobalBool("silent"):
		output.Level = output.SilentLevel
	}
}
