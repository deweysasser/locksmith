package connection

import (
	"github.com/deweysasser/locksmith/keys"
)

type Remote interface {
	RetrieveKeys() []keys.Key
}
