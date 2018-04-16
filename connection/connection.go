package connection

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/oldlib"
	"fmt"
	"os"
)

type SSHFileConnection struct {
	path string
}

func (c *SSHFileConnection) Fetch(alib *oldlib.Accountlib, klib *oldlib.KeyLib){
	fmt.Println("Reading", c.path)
	k := data.Read(c.path)
	klib.Ingest(k)
}


type Connection interface {
	Fetch(alib *oldlib.Accountlib, klib *oldlib.KeyLib)
}

func Create(a string) Connection {
	if info, _ := os.Stat(a); info != nil {
		return &SSHFileConnection{a}
	} else {
		return &SSHHostConnection{a}
	}
}
