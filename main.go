package main

import (
	"os"

	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/output"
)

func main() {

	app := cli.NewApp()
	app.Name = Name
	app.Version = Version
	app.Author = "Dewey Sasser"
	app.Email = ""
	app.Usage = ""

	app.Flags = GlobalFlags
	app.Commands = Commands
	app.CommandNotFound = CommandNotFound

	app.Run(os.Args)

	if output.ErrorCount() >0 {
		os.Exit(1)
	}
}
