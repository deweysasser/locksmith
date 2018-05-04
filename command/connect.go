package command

import (
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/lib"
	"github.com/urfave/cli"
	"os"
	"strings"
)

func CmdConnect(c *cli.Context) error {
	ml := lib.MainLibrary{Path: datadir(c)}
	clib := ml.Connections()

	for _, a := range c.Args() {

		conn := NewConnection(a, c)
		clib.Store(conn)
	}

	return nil
}

/** Determine the proper type of connection from the string given and create it
 */
func NewConnection(a string, c *cli.Context) connection.Connection {
	info, _ := os.Stat(a)

	switch {
	case info != nil:
		return &connection.FileConnection{"FileConnection", a}
	case strings.HasPrefix(a, "aws:"):
		return &connection.AWSConnection{"AWSConnection", a[4:]}
	default:
		sudo := !c.Bool("no-sudo") &&
			(c.Bool("sudo") ||
				strings.HasPrefix(a, "ubuntu@") ||
				strings.HasPrefix(a, "root@") ||
				strings.HasPrefix(a, "ec2-user@"))

		return &connection.SSHHostConnection{
			"SSHHostConnection",
			a,
			sudo,
		}
	}
}
