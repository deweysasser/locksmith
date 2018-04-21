package connection

import (
	"github.com/deweysasser/locksmith/data"
	"fmt"
	"os"
	"io/ioutil"
	"regexp"
)

type FileConnection struct {
	Type string
	Path string
}

func (c *FileConnection) String() string {
	return "file://" + c.Path
}

func (c *FileConnection) Fetch() (keys chan data.Key, accounts chan data.Account){
	fKeys := data.NewFanInKey()
	defer fKeys.DoneAdding()
	keys = fKeys.Output()

	accounts = make(chan data.Account)
	defer close(accounts)

	path := c.Path

	fetchPath(path, fKeys)
	return
}

func fetchPath(path string, inKeys *data.FanInKeys)  {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
		files, err := ioutil.ReadDir(path)
		if err == nil {
			for  _, file := range (files) {
				if !shouldSkipFile(file) {
					fetchPath(path+"/"+file.Name(), inKeys)
				}
			}
		}
	} else {
		inKeys.Add(fetchFile(path))
	}
}

func shouldSkipFile(info os.FileInfo) bool {
	switch {
	case matches("~$", info.Name()):
		return true
	case matches("^#.*", info.Name()):
		return true
	default:
		return false
	}
}

func matches(re, name string) bool {
	m, _ := regexp.Match(re, []byte(name))
	return m
}

func fetchFile(path string) chan data.Key {
	keys := make(chan data.Key)
	go func() {
		defer close(keys)
		fmt.Println("Reading", path)
		k := data.Read(path)
		
		if k != nil {
			keys <- k
		}
	}()
	return keys
}

func (c *FileConnection) Id() data.ID {
	return data.IdFromString(c.Path)
}

