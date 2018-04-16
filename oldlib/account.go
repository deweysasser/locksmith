package oldlib

import (
	"github.com/deweysasser/locksmith/data"
	"fmt"
	"os"
	"encoding/json"
	"io/ioutil"
)

type KeyBinding struct {
Id data.KeyID
//	Options []string
Comment string
}

type Account struct {
Type string
Name string
Keys []KeyBinding
lib *Accountlib
}

func (account *Account) SetKeys(keylist []data.Key) {
	bindings := make([]KeyBinding, 0)
	for _, k := range(keylist) {
		sk := k.(*data.SSHKey)
		bindings = append(bindings,
			KeyBinding{Id: k.Id(),
				//				Options: sk.Options,
				Comment: sk.Comments[0]})
	}
	account.Keys = bindings
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
