package connection

import (
	"github.com/deweysasser/locksmith/data"
)

type Remote interface {
	RetrieveKeys() []data.Key
}
