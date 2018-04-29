package connection

import (
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
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

func (c *FileConnection) Fetch() (keys <- chan data.Key, accounts <- chan data.Account) {
	fKeys := data.NewFanInKey(nil)
	defer fKeys.DoneAdding()
	keys = fKeys.Output()

	cAccounts := make(chan data.Account)
	defer close(cAccounts)

	path := c.Path

	fetchPath(path, fKeys)
	return keys, cAccounts
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
		output.Debug("Reading", path)

		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return
		}

		s := string(bytes)

		switch {
		case strings.Contains(s, "aws_access_key_id"):
			data.ParseAWSCredentials(bytes, keys)
		default:
			readSSHKey(bytes, keys, basename(path))
		}
	}()
	return keys
}

func basename(path string) string {
	if i := strings.LastIndex(path, "/"); i > 0 {
		i++
		return path[i:]
	} else {
		return path
	}
}

func readSSHKey(bytes []byte, keys chan data.Key, names ...string) {
	k := data.NewKey(string(bytes), names...)
	if k != nil {
		keys <- k
	}
}

func (c *FileConnection) Id() data.ID {
	return data.IdFromString(c.Path)
}
