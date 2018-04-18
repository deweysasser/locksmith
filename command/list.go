package command

import (
	"github.com/urfave/cli"
	"fmt"
	"strings"
	"github.com/deweysasser/locksmith/lib"
)

func CmdList(c *cli.Context) error {
	ml := lib.MainLibrary{Path: datadir()}


	filter := func(a string) bool {
		return true
	}

	args := c.Args()
	
	if len(args) > 0 {
		filter =  func(a string) bool {
			for _, s := range(args) {
				if(strings.Contains(a, s)) {
					return true
				}
			}
			return false
		}
	}

	for i:= range ml.Connections().List() {
		s := fmt.Sprintf("%s", i)
		if filter(s) {
			fmt.Println(s)
		}
	}

	return nil
}
