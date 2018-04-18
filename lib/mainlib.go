package lib

import (
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
)

type MainLibrary struct {
	Path string
	connections Library
	keys Library
	accounts Library
}

func (l *MainLibrary) Connections() Library {
	if l.connections == nil {
		clib := new(library)
		clib.Init(l.Path + "/connections", nil, connection.Deserialize)
		l.connections = clib
	}

	return l.connections
}

func (l *MainLibrary) Keys() Library {
	if l.keys == nil {
		klib := new(library)
		klib.Init(l.Path + "/keys", keyid, loadkey)
		l.keys = klib
	}

	return l.keys
}

func (l *MainLibrary) Accounts() Library {
	if l.accounts == nil {
		klib := new(library)
		klib.Init(l.Path + "/accounts", accountid, loadaccount)
		l.accounts = klib
	}

	return l.accounts
}

func loadaccount(id string, bytes []byte) (interface{}, error) {
	return data.LoadAccount(bytes)
}

func loadkey(id string, bytes []byte) (interface{}, error) {
	k := data.ReadJson(bytes)

	return k , nil
}

func accountid(account interface{}) string {
	return hashString(account.(data.Account).Name)
}
func keyid(key interface{}) string {
	return string(key.(data.Key).Id())
}
