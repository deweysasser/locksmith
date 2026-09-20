// Package history records what locksmith did, as distinct from what locksmith
// currently believes.
//
// The libraries under lib/ hold present state: this key exists, it is bound to
// that account. State cannot answer "when did this change?", and once bindings
// became able to drop -- see data.mergeBindings -- the answer stopped being
// recoverable from the stored objects at all. This package is where that
// information goes instead.
//
// Two decisions shape the format.
//
// Only *transitions* are recorded, never observations. A fetch confirms every
// binding it finds, so logging observations would write one line per binding
// per run: tens of thousands of lines a day, of which essentially all say
// "still the same". A transition log stays readable, and on a steady fleet most
// runs write nothing at all.
//
// The file is JSON Lines, one event per line, appended and never rewritten.
// That survives being kept in git -- which the README invites -- because
// appends from two machines merge cleanly where edits to a shared state file
// would conflict.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/deweysasser/locksmith/data"
)

// Event kinds.
const (
	// KeyDiscovered is written the first time a key is ever seen, not every
	// time it is seen again.
	KeyDiscovered = "key.discovered"

	// BindingAdded and BindingRemoved record a key appearing on, or
	// disappearing from, an account.  BindingRemoved is only computable
	// because a fetch can now assert what it observed.
	BindingAdded   = "binding.added"
	BindingRemoved = "binding.removed"

	// KeyExpired and KeyUnexpired record a change of intent by the operator.
	KeyExpired   = "key.expired"
	KeyUnexpired = "key.unexpired"

	// AppliedAdd and AppliedRemove record locksmith changing a remote machine.
	// These are the most important entries here: they are the only record that
	// the tool touched something outside itself.
	AppliedAdd    = "apply.added"
	AppliedRemove = "apply.removed"
)

// Event is one line of the log.
type Event struct {
	Time     time.Time            `json:"time"`
	Event    string               `json:"event"`
	Key      data.ID              `json:"key,omitempty"`
	KeyName  string               `json:"key_name,omitempty"`
	Account  data.ID              `json:"account,omitempty"`
	Location data.BindingLocation `json:"location,omitempty"`
	Detail   string               `json:"detail,omitempty"`
}

// Log is the history file for one run of one command.
//
// The file is created lazily, on the first event.  A run that changes nothing
// leaves no file behind, which is what keeps a daily cron job from filling the
// directory with empty records.
type Log struct {
	dir      string
	command  string
	hostname string
	started  time.Time

	mu sync.Mutex
	// path is kept separately from file so that Path still answers after
	// Close: a caller reporting what a run wrote needs it at the end, not the
	// middle.
	path string
	file *os.File
	enc  *json.Encoder
	err  error
}

// Open prepares a history log inside the given repository directory. It does
// not touch the filesystem until something is recorded.
func Open(repoPath, command string) *Log {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}

	return &Log{
		dir:      filepath.Join(repoPath, "history"),
		command:  command,
		hostname: host,
		started:  time.Now().UTC(),
	}
}

var notName = regexp.MustCompile(`\W+`)

// Name returns the file name this log will use.
//
// The runner's hostname is in the name, not just the timestamp, because these
// files are expected to be merged through git from several machines: two hosts
// running a nightly fetch at the same second would otherwise collide on a name
// and one would silently win.
func (l *Log) Name() string {
	return fmt.Sprintf("%s_%s_%s.jsonl",
		l.started.Format("20060102T150405Z"),
		notName.ReplaceAllString(l.hostname, "_"),
		notName.ReplaceAllString(l.command, "_"))
}

// open creates the file. Callers must hold l.mu.
func (l *Log) open() error {
	if l.file != nil || l.err != nil {
		return l.err
	}

	if err := os.MkdirAll(l.dir, 0700); err != nil {
		l.err = err
		return err
	}

	// Two runs of the same command on the same host in the same second are
	// unlikely but not impossible, and the cost of a collision is a silently
	// lost record.  Cross-machine collisions cannot happen at all, thanks to
	// the hostname, so the only case to handle is one this filesystem can see.
	base, suffix := l.Name(), ""
	for i := 2; ; i++ {
		path := filepath.Join(l.dir, base[:len(base)-len(".jsonl")]+suffix+".jsonl")

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_APPEND, 0600)
		if err == nil {
			l.file, l.path = f, path
			l.enc = json.NewEncoder(f)
			return nil
		}
		if !os.IsExist(err) {
			l.err = err
			return err
		}
		if i > 100 {
			l.err = fmt.Errorf("could not find an unused history file name for %s", base)
			return l.err
		}
		suffix = fmt.Sprintf("_%d", i)
	}
}

// Record appends an event. It is safe to call from several goroutines, which
// it has to be: ingestion runs concurrently with fetching.
//
// Recording is best-effort. History is a record of work, not the work itself,
// and a failure to write it must never abort a fetch or an apply; the error is
// returned for a caller that wants to report it, and otherwise ignored.
func (l *Log) Record(e Event) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.open(); err != nil {
		return err
	}

	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}

	return l.enc.Encode(e)
}

// Close closes the underlying file, if one was ever created.
func (l *Log) Close() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}

	f := l.file
	l.file, l.enc = nil, nil
	return f.Close()
}

// Path returns the file this log wrote to, or "" if it never wrote anything.
// It remains valid after Close.
func (l *Log) Path() string {
	if l == nil {
		return ""
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.path
}
