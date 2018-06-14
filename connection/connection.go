package connection

import (
	"github.com/deweysasser/locksmith/data"
)

type Connection interface {
	Fetch() (keys <-chan data.Key, accounts <-chan data.Account)
	Id() data.ID
}

type Changer interface {
	Update(account data.Account, addBindings []data.KeyBindingImpl, removeBindings []data.KeyBindingImpl, keylib data.KeyFetcher) error
}
