package connection

import (
	"github.com/deweysasser/locksmith/data"
)

type Connection interface {
	Fetch() (keys <-chan data.Key, accounts <-chan data.Account)
	Id() data.ID
}

type Changer interface {
	Add(account *data.Account, bindings []data.KeyBinding)
	Remove(account *data.Account, bindings []data.KeyBinding)
}
