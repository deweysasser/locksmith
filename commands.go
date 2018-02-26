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
		Name:   "add",
		Usage:  "",
		Action: command.CmdAdd,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "list",
		Usage:  "",
		Action: command.CmdList,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "pull",
		Usage:  "",
		Action: command.CmdPull,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "push",
		Usage:  "",
		Action: command.CmdPush,
		Flags:  []cli.Flag{},
	},
}

func CommandNotFound(c *cli.Context, command string) {
	fmt.Fprintf(os.Stderr, "%s: '%s' is not a %s command. See '%s --help'.", c.App.Name, command, c.App.Name, c.App.Name)
	os.Exit(2)
}
