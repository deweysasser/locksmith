package main

import (
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

const Name string = "locksmith"
const Version string = "development"


func CmdVersion(c *cli.Context) error {

	output.Normal(Version)
	return nil
}
