package connection

import (
	"encoding/json"
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

func Deserialize(id string, bytes []byte) (interface{}, error) {
	t := make(map[string]interface{})
	json.Unmarshal(bytes, &t)
	var n Connection

	switch t["Type"] {
	case "FileConnection":
		n = new(FileConnection)
	case "SSHHostConnection":
		n = new(SSHHostConnection)
	}

	e := json.Unmarshal(bytes, n)
	if e != nil {
		return nil, e
	}
	return n, nil
}
