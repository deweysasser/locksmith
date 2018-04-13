package command

import (
	"github.com/urfave/cli"
	"github.com/deweysasser/locksmith/lib"	
	"fmt"
	"strings"
)

func CmdList(c *cli.Context) error {
	keylib := lib.NewKeylib(datadir())
	accountlib := lib.NewAccountlib(datadir())

	keys, _ := keylib.AllKeys()
	accounts, _ := accountlib.GetAccounts()

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

	for key := range keys {
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
