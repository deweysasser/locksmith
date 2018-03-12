package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/keylib"	
	"fmt"
	"strings"
)

func CmdList(c *cli.Context) error {
	lib := keylib.KeyLib{datadir()}

	keys, _ := lib.Keys()


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

	for _, key := range keys {
		s := fmt.Sprintf("%s", key)
		if filter(s) {
			fmt.Println(s)
		}
	}

	return nil
}
