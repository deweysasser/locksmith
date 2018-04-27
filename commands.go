package main

import (
	"fmt"
	"os"

	"github.com/deweysasser/locksmith/command"
	"github.com/urfave/cli"
)

var GlobalFlags = []cli.Flag{
	cli.BoolFlag{
		Name: "debug, d",
		Usage: "Debug output",
	},
}

var outputFlags = []cli.Flag{
	cli.BoolFlag{
		Name:"verbose, v",
		Usage: "Verbose output",
	},
	cli.BoolFlag{
		Name: "debug, d",
		Usage: "Debug output",
	},
}


var Commands = []cli.Command{
	{
		Name:   "connect",
		Usage:  "Connect a new source of keys",
		Action: command.CmdConnect,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "fetch",
		Usage:  "fetch keys from sources",
		Action: command.CmdFetch,
		Flags:  outputFlags,
	},
	{
		Name:   "list",
		Usage:  "list all objects",
		Action: command.CmdList,
		Flags:  outputFlags,
	},
	{
		Name:   "remove",
		Usage:  "remove the given objects",
		Action: command.CmdRemove,
		Flags:  []cli.Flag{},
	},
	{
		Name: "add-id",
		Usage: "Add an ID to 1 key",
		Action: command.CmdAddId,
		Flags: []cli.Flag{},
	},
}
}

func CommandNotFound(c *cli.Context, command string) {
	fmt.Fprintf(os.Stderr, "%s: '%s' is not a %s command. See '%s --help'.", c.App.Name, command, c.App.Name, c.App.Name)
	os.Exit(2)
}
