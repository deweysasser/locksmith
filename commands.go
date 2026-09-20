package main

import (
	"fmt"
	"os"

	"github.com/deweysasser/locksmith/command"
	"github.com/urfave/cli"
)

var GlobalFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "debug, d",
		Usage: "Debug output",
	},
	cli.StringFlag{
		Name:  "repo, r",
		Usage: "Location of locksmith repository",
	},
}

var outputFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "verbose, v",
		Usage: "Verbose output",
	},
	cli.BoolFlag{
		Name:  "debug, d",
		Usage: "Debug output",
	},
}

var Commands = []cli.Command{
	{
		Name:   "connect",
		Usage:  "Connect a new source of keys",
		Action: command.CmdConnect,
		Flags: append(outputFlags,
			cli.BoolFlag{
				Name:  "sudo, s",
				Usage: "Use sudo to retrieve keys from all accounts",
			},
			cli.BoolFlag{
				Name:  "no-sudo",
				Usage: "Do *NOT* use sudo even for root, ubuntu or ec2-user users",
			},
		),
	},
	{
		Name:   "fetch",
		Usage:  "fetch keys from sources",
		Action: command.CmdFetch,
		Flags: append(outputFlags,
			cli.DurationFlag{
				Name:  "timeout, t",
				Usage: "give up on the whole fetch after this long (e.g. 5m); 0 means no limit",
			}),
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
		Flags:  outputFlags,
	},
	{
		Name:   "add-id",
		Usage:  "Add an ID to 1 key",
		Action: command.CmdAddId,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "display-lib",
		Usage:  "Display the library for debugging purposes",
		Action: command.CmdDisplayLib,
		Flags:  []cli.Flag{},
	},
	{
		Name:   "expire",
		Usage:  "Mark the matching keys to be removed from every system",
		Action: command.CmdExpire,
		Flags: append(outputFlags,
			cli.StringFlag{
				Name:  "replace-with, r",
				Usage: "replace the matching keys with this one, rather than only removing them",
			}),
	},
	{
		Name:   "unexpire",
		Usage:  "Undo `expire` for the matching keys",
		Action: command.CmdUnexpire,
		Flags:  outputFlags,
	},
	{
		Name:   "plan",
		Usage:  "Calculate changes",
		Action: command.CmdPlan,
		Flags:  outputFlags,
	},
	{
		Name:   "add",
		Usage:  "Add a key or keys to an account or accounts",
		Action: command.CmdAdd,
		Flags: append(outputFlags,
			cli.StringFlag{
				Name:  "key, k",
				Usage: "key or keys to add",
			}),
	},
	{
		Name:   "apply",
		Usage:  "Apply pending changes",
		Action: command.CmdApply,
		Flags: append(outputFlags,
			cli.BoolFlag{
				Name:  "dry-run, n",
				Usage: "print the commands that would run on each host, and run none of them",
			}),
	},
}

func CommandNotFound(c *cli.Context, command string) {
	fmt.Fprintf(os.Stderr, "%s: '%s' is not a %s command. See '%s --help'.", c.App.Name, command, c.App.Name, c.App.Name)
	os.Exit(2)
}
