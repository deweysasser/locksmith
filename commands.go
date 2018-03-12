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
		Name:   "refresh",
		Usage:  "",
		Action: command.CmdRefresh,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "deploy",
		Usage:  "",
		Action: command.CmdDeploy,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "deprecate",
		Usage:  "",
		Action: command.CmdDeprecate,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "forget",
		Usage:  "",
		Action: command.CmdForget,
		Flags:  []cli.Flag{},
	},
}

func CommandNotFound(c *cli.Context, command string) {
	fmt.Fprintf(os.Stderr, "%s: '%s' is not a %s command. See '%s --help'.", c.App.Name, command, c.App.Name, c.App.Name)
	os.Exit(2)
}
