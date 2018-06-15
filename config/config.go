package config

import (
	"github.com/deweysasser/locksmith/output"
	"os"
	"github.com/urfave/cli"
	"reflect"
	"time"
)


type properties struct {
	LOCKSMITH_REPO string
	LOCKSMITH_SSH string
	NOW time.Time
}

var Property properties

type PropertyValue struct {
	Name, Value string
}

// Return the property fields
func Properties() <-chan PropertyValue{
	c := make(chan PropertyValue)

	go func() {
		defer close(c)
		v:= reflect.ValueOf(Property)
		t:= reflect.TypeOf(Property)

		for i := 0; i< t.NumField(); i++ {
			c <- PropertyValue{t.Field(i).Name, v.Field(i).String()}
		}
	}()

	return c
}

var propType reflect.Type = reflect.TypeOf(Property)

func Init(c *cli.Context) {
	switch {
	case c.Bool("debug") || c.GlobalBool("debug"):
		output.Level = output.DebugLevel
	case c.Bool("verbose") || c.GlobalBool("verbose"):
		output.Level = output.VerboseLevel
	case c.Bool("silent") || c.GlobalBool("silent"):
		output.Level = output.SilentLevel
	}

	Property = properties{
		datadir(c),
		getSshCommand(),
		time.Now(),
	}
}


// Find the locksmith data directory given flags and environment
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


// Return the SSH command to use
func getSshCommand() string {
	if ssh, ok := os.LookupEnv("LOCKSMITH_SSH"); ok {
		output.Debug("Using command from LOCKSMITH_SSH: ", ssh)
		return ssh
	} else {
		return "ssh"
	}
}
