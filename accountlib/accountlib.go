package accountlib

import (
	"fmt"
	"github.com/deweysasser/locksmith/keys"
//	"io/ioutil"
	"os"
//	"regexp"
)

type Accountlib struct {
	Path string
}

type Account struct {
	Type string
}

func New(path string) *Accountlib {
	return &Accountlib{path}
}

func (lib *Accountlib) EnsureAccount(name string) *Account {
	return &Account{"ssh"}
}

func (account *Account) SetKeys([]keys.Key) {
	return
}

func check(reason string, e error) {
	if e != nil {
		panic(fmt.Sprintf("%s: %s", reason, e))
	}
}


func (lib *Accountlib) accountpath() string {
	accountpath := lib.Path + "/accounts"
	_, err := os.Stat(accountpath)

	if err != nil {
		e := os.MkdirAll(accountpath, 755)
		check("Failed to create dir", e)
	}

	return accountpath
}


