package command

import (
	"fmt"
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

	home := os.Getenv("HOME")
	r := home + "/" + ".x-locksmith"
	output.Debug("Repo in home directory:", r)
	return r
}

type Filter func(interface{}) bool

func buildFilterFromContext(c *cli.Context) Filter {
	return buildFilter(c.Args())
}

func buildFilter(args []string) Filter {
	filter := func(a interface{}) bool {
		return true
	}

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

func outputLevel(c *cli.Context) {
	switch {
	case c.Bool("debug"):
		output.Level = output.DebugLevel
	case c.Bool("verbose"):
		output.Level = output.VerboseLevel
	case c.Bool("silent"):
		output.Level = output.SilentLevel
	}
}
