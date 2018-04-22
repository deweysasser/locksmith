package connection

import (
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

type FileConnection struct {
	Type string
	Path string
}

func (c *FileConnection) String() string {
	return "file://" + c.Path
}

func (c *FileConnection) Fetch() (keys chan data.Key, accounts chan data.Account) {
	fKeys := data.NewFanInKey(nil)
	defer fKeys.DoneAdding()
	keys = fKeys.Output()

	accounts = make(chan data.Account)
	defer close(accounts)

	path := c.Path

	fetchPath(path, fKeys)
	return
}

func fetchPath(path string, inKeys *data.FanInKeys) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
		files, err := ioutil.ReadDir(path)
		if err == nil {
			for _, file := range files {
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

		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return
		}

		s := string(bytes)

		switch {
		case strings.Contains(s, "aws_access_key_id"):
			data.ParseAWSCredentials(bytes, keys)
		default:
			readSSHKey(bytes, keys)
		}
	}()
	return keys
}

func readSSHKey(bytes []byte, keys chan data.Key) {
	k := data.New(string(bytes))
	if k != nil {
		keys <- k
	}
}

func (c *FileConnection) Id() data.ID {
	return data.IdFromString(c.Path)
}
