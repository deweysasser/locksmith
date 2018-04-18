package connection

import (
	"github.com/deweysasser/locksmith/data"
	"fmt"
	"os"
	"encoding/json"
)

type SSHFileConnection struct {
	Type string
	Path string
}

func (c *SSHFileConnection) Fetch(keys chan data.Key, accounts chan data.Account){
	fmt.Println("Reading", c.Path)
	k := data.Read(c.Path)
	keys <- k
}


type Connection interface {
	Fetch(keys chan data.Key, accounts chan data.Account)
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
		n =  new(SSHHostConnection)
	}

	e := json.Unmarshal(bytes, n)
	if e != nil {
		return nil, e
	}
	return n, nil
}
