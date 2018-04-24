package main

import (
	"fmt"
	"os"

	"github.com/deweysasser/locksmith/command"
	"github.com/urfave/cli"
)

var GlobalFlags = []cli.Flag{}

var Commands = []cli.Command{
	{
		Name:   "connect",
		Usage:  "",
		Action: command.CmdConnect,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "fetch",
		Usage:  "",
		Action: command.CmdFetch,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "list",
		Usage:  "",
		Action: command.CmdList,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "rm",
		Usage:  "",
		Action: command.CmdRemove,
		Flags:  []cli.Flag{},
	},
}

func CommandNotFound(c *cli.Context, command string) {
	fmt.Fprintf(os.Stderr, "%s: '%s' is not a %s command. See '%s --help'.", c.App.Name, command, c.App.Name, c.App.Name)
	os.Exit(2)
}
