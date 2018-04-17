package connection

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/oldlib"
	"fmt"
	"os"
	"encoding/json"
)

type SSHFileConnection struct {
	Type string
	Path string
}

func (c *SSHFileConnection) Fetch(alib *oldlib.Accountlib, klib *oldlib.KeyLib){
	fmt.Println("Reading", c.Path)
	k := data.Read(c.Path)
	klib.Ingest(k)
}


type Connection interface {
	Fetch(alib *oldlib.Accountlib, klib *oldlib.KeyLib)
}

func Create(a string) Connection {
	if info, _ := os.Stat(a); info != nil {
		return &SSHFileConnection{ "SSHFileConnection", a}
	} else {
		return &SSHHostConnection{ "SSHHostConnection", a}
	}
}

func Deserialize(id string, bytes []byte) (interface{}, error) {
	t:= make(map[string]interface{})
	json.Unmarshal(bytes, &t)
	var n Connection

	switch t["Type"] {
	case "SSHFileConnection":
		n = new(SSHFileConnection)
	case "SSHHostConnection":
		n =  new(SSHFileConnection)
	}

	e := json.Unmarshal(bytes, n)
	if e != nil {
		return nil, e
	}
	return n, nil
}
