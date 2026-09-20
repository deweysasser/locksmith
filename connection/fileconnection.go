package connection

import (
	"context"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"
)

type FileConnection struct {
	Type string
	Path string
}

func (c *FileConnection) String() string {
	return "file://" + c.Path
}

func (c *FileConnection) Fetch(ctx context.Context) (keys <-chan data.Key, accounts <-chan data.Account) {
	fKeys := data.NewFanInKey(nil)
	defer fKeys.DoneAdding()
	keys = fKeys.Output()

	cAccounts := make(chan data.Account)
	defer close(cAccounts)

	path := c.Path

	fetchPath(ctx, path, fKeys)
	return keys, cAccounts
}

func fetchPath(ctx context.Context, path string, inKeys *data.FanInKeys) {
	// A large tree is a lot of files; check before each one so a cancelled
	// fetch stops walking rather than finishing the directory first.
	if ctx.Err() != nil {
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
		files, err := ioutil.ReadDir(path)
		if err == nil {
			for _, file := range files {
				if !shouldSkipFile(file) {
					fetchPath(ctx, path+"/"+file.Name(), inKeys)
				}
			}
		}
	} else {
		inKeys.Add(fetchFile(ctx, path))
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

func fetchFile(ctx context.Context, path string) chan data.Key {
	keys := make(chan data.Key)
	go func() {
		defer close(keys)
		output.Debug("Reading", path)

		if info, err := os.Stat(path); err == nil {
			bytes, err := ioutil.ReadFile(path)
			if err != nil {
				return
			}

			s := string(bytes)

			switch {
			case strings.Contains(s, "aws_access_key_id"):
				data.ParseAWSCredentials(bytes, keys)
			case strings.Contains(path, "known_hosts"):
				output.Debug("Skipping known hosts file", path)
			default:
				readSSHKey(ctx, bytes, keys, info.ModTime(), basename(path))
			}
		} else {
			output.Error("Failed to read", path)
		}
	}()
	return keys
}

func basename(path string) string {
	// >= 0, not > 0: a path at the filesystem root ("/rsa.pub") has its
	// separator at index 0 and still has a name of its own.
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	} else {
		return path
	}
}

func readSSHKey(ctx context.Context, bytes []byte, keys chan data.Key, time time.Time, names ...string) {
	// NewKeys, not NewKey: a file may hold many entries, and taking only the
	// first silently drops the rest from the catalogue.
	for _, k := range data.NewKeys(string(bytes), time, names...) {
		// The consumer may have gone away; without this select the goroutine
		// blocks on the send forever and leaks.
		select {
		case keys <- k:
		case <-ctx.Done():
			return
		}
	}
}

func (c *FileConnection) Id() data.ID {
	return data.IdFromString(c.Path)
}
