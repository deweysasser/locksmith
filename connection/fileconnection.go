package connection

import (
	"github.com/deweysasser/locksmith/data"
	"fmt"
)

type FileConnection struct {
	Type string
	Path string
}

func (c *FileConnection) String() string {
	return "file://" + c.Path
}

func (c *FileConnection) Fetch() (keys chan data.Key, accounts chan data.Account){
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

func (c *FileConnection) Id() data.ID {
	return data.IdFromString(c.Path)
}

