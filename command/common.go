package command

import (
	"os"
	"strings"
	"fmt"
)

// Return the locksmith data directory
func datadir() string {
	home := os.Getenv("HOME")
	return home + "/" + ".x-locksmith"
}

func buildFilter(args []string) func (interface{}) bool {
	filter := func(a interface{}) bool {
		return true
	}


	if len(args) > 0 {
		filter =  func(i interface{}) bool {
			a := fmt.Sprintf("%s", i)
			for _, s := range(args) {
				if(strings.Contains(a, s)) {
					return true
				}
			}
			return false
		}
	}

	return filter
}