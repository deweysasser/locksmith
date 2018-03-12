package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/keylib"	
	"github.com/deweysasser/locksmith/accountlib"	
	"fmt"
	"strings"
)

func CmdList(c *cli.Context) error {
	lib := keylib.New(datadir())
	accountlib := accountlib.New(datadir())

	keys, _ := lib.Keys()
	accounts, _ := accountlib.Accounts()

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

	for _, account := range accounts {
		s := fmt.Sprintf("%s", account)
		if filter(s) {
			fmt.Println(s)
		}
	}

	return nil
}
