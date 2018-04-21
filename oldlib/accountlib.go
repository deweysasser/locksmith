package oldlib

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	//	"regexp"
)

type Accountlib struct {
	library
	Accounts []Account
}

func NewAccountlib(path string) *Accountlib {
	return &Accountlib{library{path}, []Account{}}
}

func (lib *Accountlib) EnsureAccount(name string) *Account {
	a := Account{Name: name, Type: "SSH"}

	lib.Accounts = append(lib.Accounts, a)

	return &a
}

func (lib *Accountlib) GetAccounts() ([]Account, error) {
	adir := lib.Path + "/accounts"

	_, error := os.Stat(adir)

	if error != nil {
		return []Account{}, nil
	}

	files, error := ioutil.ReadDir(lib.Path + "/accounts")

	fmt.Println("Reading files in ", lib.Path)

	check("Failed to read accounts dir", error)

	for _, f := range files {
		dir := lib.Path + "/accounts/" + f.Name()

		fmt.Println("Reading", dir)

		acctfiles, error := ioutil.ReadDir(dir)
		check("Failed to read TYPE dir", error)

		for _, f2 := range acctfiles {
			lib.Read(dir + "/" + f2.Name())
		}
	}

	return lib.Accounts, nil
}

func (lib *Accountlib) Read(file string) {
	data, e := ioutil.ReadFile(file)

	check("Failed to read account file "+file, e)

	var acc Account
	json.Unmarshal(data, &acc)

	lib.Accounts = append(lib.Accounts, acc)
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
