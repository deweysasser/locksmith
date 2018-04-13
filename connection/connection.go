package connection

import (
	"github.com/deweysasser/locksmith/keys"
	"github.com/deweysasser/locksmith/lib"
	"fmt"
	"os"
)

type SSHFileConnection struct {
	path string
}

func (c *SSHFileConnection) Fetch(alib *lib.Accountlib, klib *lib.KeyLib){
	fmt.Println("Reading", c.path)
	k := keys.Read(c.path)
	klib.Ingest(k)
}


type Connection interface {
	Fetch(alib *lib.Accountlib, klib *lib.KeyLib)
}

func Create(a string) Connection {
	if info, _ := os.Stat(a); info != nil {
		return &SSHFileConnection{a}
	} else {
		return &SSHHostConnection{a}
	}
}
