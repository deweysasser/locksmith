package connection

import (
	"context"

	"github.com/deweysasser/locksmith/data"
)

type Connection interface {
	// Fetch streams the keys and accounts this connection can see.
	//
	// The context is the caller's way out.  Every implementation here talks to
	// something it does not control -- an SSH session, a cloud API, a
	// filesystem -- and without cancellation a single unresponsive host pins
	// the whole run: `fetch` across a fleet would hang with no way to abort but
	// killing the process, which leaves orphaned ssh children behind.
	//
	// Both channels are closed when the fetch finishes, whether it completed or
	// was cancelled.  A cancelled fetch reports what it had already found.
	Fetch(ctx context.Context) (keys <-chan data.Key, accounts <-chan data.Account)
	Id() data.ID
}

type Changer interface {
	Update(account data.Account, addBindings []data.KeyBindingImpl, removeBindings []data.KeyBindingImpl, keylib data.Fetcher) error
}
