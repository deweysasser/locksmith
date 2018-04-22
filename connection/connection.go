package connection

import (
	"github.com/deweysasser/locksmith/data"
	"os"
)

type Connection interface {
	Fetch() (keys chan data.Key, accounts chan data.Account)
	Id() data.ID
}

/** Determine the proper type of connection from the string given and create it
 */
func Create(a string) Connection {
	if info, _ := os.Stat(a); info != nil {
		return &FileConnection{"FileConnection", a}
	} else {
		return &SSHHostConnection{"SSHHostConnection", a}
	}
}
