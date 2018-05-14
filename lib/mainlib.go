package lib

import (
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"reflect"
)

type MainLibrary struct {
	Path        string
	connections ConnectionLibrary
	keys        KeyLibrary
	accounts    AccountLibrary
	changes     ChangeLibrary
}

func init() {
	AddType(reflect.TypeOf(data.SSHKey{}))
	AddType(reflect.TypeOf(data.AWSKey{}))
	AddType(reflect.TypeOf(connection.SSHHostConnection{}))
	AddType(reflect.TypeOf(connection.FileConnection{}))
	AddType(reflect.TypeOf(connection.AWSConnection{}))
	AddType(reflect.TypeOf(data.SSHAccount{}))
	AddType(reflect.TypeOf(data.AWSAccount{}))
	AddType(reflect.TypeOf(data.AWSInstanceAccount{}))
	AddType(reflect.TypeOf(data.AWSIamAccount{}))
	AddType(reflect.TypeOf(data.Change{}))
}

func (l *MainLibrary) Connections() ConnectionLibrary {
	if l.connections == nil {
		clib := NewConnectionLibrary(l.Path + "/connections")
		l.connections = clib
	}

	return l.connections
}

func (l *MainLibrary) Keys() KeyLibrary {
	if l.keys == nil {
		klib := NewKeyLibrary(l.Path + "/keys")
		l.keys = klib
	}

	return l.keys
}

func (l *MainLibrary) Accounts() AccountLibrary {
	if l.accounts == nil {
		klib := NewAccountLibrary(l.Path + "/accounts")
		l.accounts = klib
	}

	return l.accounts
}

func (l *MainLibrary) Changes() ChangeLibrary {
	if l.changes == nil {
		klib := NewChangeLibrary(l.Path + "/changes")
		l.changes = klib
	}

	return l.changes
}

type keyReadError struct {
	Path string
}

func (e *keyReadError) Error() string {
	return "Error reading " + e.Path
}

func keyid(key interface{}) string {
	return string(key.(data.Key).Id())
}
