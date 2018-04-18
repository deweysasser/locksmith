package lib

import (
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
)

type MainLibrary struct {
	Path string
	connections Library
	keys Library
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

func loadkey(id string, bytes []byte) (interface{}, error) {
	k := data.ReadJson(bytes)

	return k , nil
}

func keyid(key interface{}) string {
	return string(key.(data.Key).Id())
}
