package connection

import (
	"github.com/deweysasser/locksmith/data"
	"os"
	"strings"
)

type Connection interface {
	Fetch() (keys <- chan data.Key, accounts <- chan data.Account)
	Id() data.ID
}

/** Determine the proper type of connection from the string given and create it
 */
func Create(a string) Connection {
	info, _ := os.Stat(a)

	switch {
	case info != nil:
		return &FileConnection{"FileConnection", a}
	case strings.HasPrefix(a, "aws:"):
		return &AWSConnection{"AWSConnection", a[4:]}
	default:
		return &SSHHostConnection{"SSHHostConnection", a}
	}
}
