package accountlib

import (
	"fmt"
	"github.com/deweysasser/locksmith/keys"
	"encoding/json"
	"io/ioutil"
	"os"
//	"regexp"
)

type Accountlib struct {
	Path string
}

type KeyBinding struct {
	Id string
	Options []string
	Comment string
}

type Account struct {
	Name string
	Type string
	Keys []KeyBinding
	lib *Accountlib
}

func New(path string) *Accountlib {
	return &Accountlib{path}
}

func (lib *Accountlib) EnsureAccount(name string) *Account {
	a := &Account{Name: name, Type: "SSH"}

	a.lib = lib

	return a
}

func (account *Account) SetKeys(keylist []keys.Key) {
	bindings := make([]KeyBinding, 0)
	for _, k := range(keylist) {
		sk := k.(*keys.SSHPublicKey)
		bindings = append(bindings,
			KeyBinding{Id: k.Id(),
				Options: sk.Options,
				Comment: sk.Comments[0]})
	}
	account.Keys = bindings
	account.Save()
	return
}

func (account *Account) Save() {
	path := fmt.Sprintf("%s/%s", account.lib.accountpath(),
		account.Type)

	_, err := os.Stat(path)

	if err != nil {
		e := os.MkdirAll(path, 755)
		check("Failed to create dir", e)
	}

	file := fmt.Sprintf("%s/%s.json", path, account.Name)

	json, err := json.MarshalIndent(account, "", "  ")

	check("Failed to marshal account", err)

	ioutil.WriteFile(file, json, 0644)

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


