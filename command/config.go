package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/config"
	"github.com/deweysasser/locksmith/output"
	"fmt"
)

func CmdEnv(c *cli.Context) error {
	config.Init(c)

	for f := range config.Properties() {
		output.Normal(fmt.Sprintf("%s=%s", f.Name, f.Value))
	}
	return nil
}