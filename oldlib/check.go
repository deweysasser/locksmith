package oldlib

import "fmt"

func check(reason string, e error) {
	if e != nil {
		panic(fmt.Sprintf("%s: %s", reason, e))
	}
}
