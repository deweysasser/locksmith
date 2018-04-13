package connection

import (
	"github.com/deweysasser/locksmith/keys"
)

type Connection interface {
	RetrieveKeys() []keys.Key
}
