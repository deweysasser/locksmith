package lib


//go:generate touch foo


import (
	"github.com/deweysasser/locksmith/output"
	"os"
	"io/ioutil"
)

type ChangeLibrary interface {
	Library
	Clear() error
}

type changelib struct {
	library
}

func (l *changelib) Clear() error {
	output.Debug("Clearing objects from", l.Path)

	if _, e := os.Stat(l.Path); e != nil {
		return e
	}

	if files, e := ioutil.ReadDir(l.Path); e == nil {
		for _, f := range files {
			path := l.Path + "/" + f.Name()
			//fmt.Println("Reading from ", path)

			if e := os.Remove(path); e != nil {
				output.Error("Failed to delete", path)
				return e
			}
		}
	}	else {
		return e
	}

	return nil
}
