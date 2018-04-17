package lib

import "github.com/deweysasser/locksmith/connection"

type MainLibrary struct {
	Path string
	connections Library
}

func (l *MainLibrary) Connections() Library {
	if l.connections == nil {
		clib := new(library)
		clib.Init(l.Path + "/connections", nil, connection.Deserialize)
		l.connections = clib
	}

	return l.connections
}
