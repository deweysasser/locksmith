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

// Previewer renders what Update would do, without doing it and without opening
// a connection.
//
// It returns the literal commands that would run on the remote host, in order.
// That is the point: `apply --dry-run` is only worth anything if what it prints
// is the same string the real path executes, so an implementation must build
// its preview with the same code Update uses rather than a second formatter
// that can drift out of step.
//
// A Changer that cannot describe itself this way is simply not a Previewer, and
// `apply --dry-run` says so rather than implying there is nothing to do.
type Previewer interface {
	Preview(account data.Account, addBindings []data.KeyBindingImpl, removeBindings []data.KeyBindingImpl, keylib data.Fetcher) ([]string, error)
}
