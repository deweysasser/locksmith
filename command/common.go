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
type filterBuilder func(s string) Filter

func buildFilterFromContext(c *cli.Context) Filter {
	return buildFilter(c.Args())
}

func mapTerms(strings []string, builder filterBuilder) []Filter {
	var filters []Filter
	for _, s := range strings {
		filters = append(filters, builder(s))
	}
	return filters
}

func AcceptAll(a interface{}) bool {
	return true
}

func build_contains(match string) Filter {
	return func(i interface{}) bool {
		a := fmt.Sprint(i)
		if strings.Contains(a, match) {
			return true
		}
		return false
	}
}


func build_and(filters []Filter) Filter {
	return func(i interface{}) bool {
		for _, f := range filters {
			if !f(i) {
				return false
			}
		}
		return true
	}
}

func build_or(filters []Filter) Filter {
	return func(i interface{}) bool {
		for _, f := range filters {
			if f(i) {
				return true
			}
		}
		return false
	}
}

func build_term_filter(term string) Filter {
	if !strings.Contains(term, "&") {
		return build_contains(term)
	} else {
		parts := strings.Split(term, "&")
		return build_and(mapTerms(parts, build_contains))
		filters := make([]Filter, 0)
		for _, p := range parts {
			filters = append(filters, build_contains(p))
		}
		return build_and(filters)
	}
}

func buildFilter(args []string) Filter {

	switch len(args) {
	case 0:
		return AcceptAll
	case 1:
		return build_term_filter(args[0])
	default:
		return build_or(mapTerms(args, build_term_filter))
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
	config.Init(c)
}
