package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"os"
	"strings"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/data"
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
	if	home := os.Getenv("HOME"); home != "" {
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

func buildFilter(args []string) Filter {
	filter := AcceptAll

	if len(args) > 0 {
		filter = func(i interface{}) bool {
			a := fmt.Sprintf("%s", i)
			for _, s := range args {
				if strings.Contains(a, s) {
					return true
				}
			}
			return false
		}
	}

	return filter
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
