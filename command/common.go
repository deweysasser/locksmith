package command

import (
	"fmt"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
	"strings"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/config"
	"time"
	"errors"
	"github.com/deweysasser/locksmith/lib"
	"reflect"
	"github.com/deweysasser/locksmith/connection"
)


type Filter func(interface{}) bool
type filterBuilder func(s string) Filter

/** The default filter -- accepts anything */
func AcceptAll(a interface{}) bool {
	return true
}

/** the oposite of AcceptAll
 */
var errorFilter Filter = build_not(AcceptAll)


func buildFilterFromContext(c *cli.Context) Filter {
	return buildFilter(c.Args())
}

/** Map a series of terms over a filter builder
 */
func mapTerms(strings []string, builder filterBuilder) []Filter {
	var filters []Filter
	for _, s := range strings {
		filters = append(filters, builder(s))
	}
	return filters
}

type FilterContext struct {
	lib lib.MainLibrary
}

/** Render the object as a string
 */
func toString(context FilterContext, i interface{}, prefix string) (string, error) {
	switch o := i.(type) {
	case data.Account:
		return accountString(o, prefix), nil
	case data.Key:
		return keyString(o, prefix), nil
	case  connection.Connection:
		return connectionString(o, prefix), nil
	case data.Change:
		return  changeString(o, context.lib.Accounts())
	default:
		return fmt.Sprintf("%s%s", prefix, i), nil
	}
}

/** build a 'contains' filter
 */
func build_contains(match string) Filter {
	output.Debug("Building contains filter around: ", match)
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

func build_not(filter Filter) Filter {
	return func(i interface{}) bool {
		return !filter(i)
	}
}

func build_age_filter(s string) Filter {
	if duration, err := parseDuration(s); err != nil {
		output.Error("Failed to parse age string:", s)
		return errorFilter
	} else {
		output.Debug("Building age filter around", duration)
		return func(i interface{}) bool {
			if a, ok := i.(data.FirstTimer); ok {
				ageOfObject := config.Property.NOW.Sub(a.FirstTime()).Hours()
				output.Debug(fmt.Sprintf("Object %s is %d hours old", i, ageOfObject))
				return ageOfObject > duration.Hours()
			} else {
				output.Debug(reflect.TypeOf(i).Name(), " is not a FirstTimer:", i)
				return false
			}
		}
	}
}

/* Parse an age specification into a duration
 */
func parseDuration(s string) (time.Duration, error) {
	var i int
	var suffix string
	n, e := fmt.Sscanf(s, "%d%s", &i, &suffix)

	if n == 1 {
		return time.Duration(i) * time.Hour * 24, nil
	} else if e == nil {
		if mult, e := getSuffix(suffix); e == nil {
			return time.Duration(i) * mult, nil
		} else {
			return 0, e
		}
	} else {
		return 0, e
	}
}

/** Parse a suffix into a duration, e.g. h=> time.Hours
 */
func getSuffix(s string) (time.Duration, error) {
	switch (s) {
	case "h":
		return time.Hour, nil
	case "d":
		return time.Hour*24, nil
	case "y":
		return time.Hour*365, nil
	default:
		return time.Nanosecond, errors.New(fmt.Sprint("Unrecognized Suffix ", s))
	}
}

/** Build a filter from an atomic filter part
 */
func  build_filter_atom(s string) Filter {
	output.Debug("Testing filter for age:", s)
	switch {
	//case strings.HasPrefix(s, "age:"):
	//	return build_age_filter(s[4:])
	default:
		return build_contains(s)
	}

}

func build_term_filter(term string) Filter {
	if !strings.Contains(term, "&") {
		return build_filter_atom(term)
	} else {
		parts := strings.Split(term, "&")
		return build_and(mapTerms(parts, build_contains))
		filters := make([]Filter, 0)
		for _, p := range parts {
			filters = append(filters, build_filter_atom(p))
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
