package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"strings"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/config"
)


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
	config.Init(c)
}
