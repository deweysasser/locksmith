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

func (c *SSHFileConnection) String() string {
	return "file://" + c.Path
}

func (c *SSHFileConnection) Fetch() (keys chan data.Key, accounts chan data.Account){
	keys = make(chan data.Key)
	accounts = make(chan data.Account)

	go func() {
		fmt.Println("Reading", c.Path)
		k := data.Read(c.Path)
		keys <- k
		close(keys)
		close(accounts)
	}()
	return
}

func (c *SSHFileConnection) Id() data.ID {
	return data.IdFromString(c.Path)
}

type Connection interface {
	Fetch() (keys chan data.Key, accounts chan data.Account)
	Id() data.ID
}

/** Determine the proper type of connection from the string given and create it
 */
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
