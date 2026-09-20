package lib

import (
	"fmt"

	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

// This file replaces four near-identical files that were generated from
// keyLibrary.go by sed substitutions in lib/Makefile.
//
// That scheme had stopped paying for itself.  The template only worked for
// element types that are interfaces, so data.Change -- a value type, which
// arrives from disk as a pointer and from the cache as a value -- could not be
// expressed by substitution.  changeLibrary.go was therefore hand-maintained,
// and its Make rule had become `test -f $@ && touch $@ || sed ...`: a build rule
// whose purpose was to not run.  Substitution was also blunt enough to be
// dangerous, since s/Key/Account/g would happily rewrite data.KeyBindingImpl
// into data.AccountBindingImpl the day someone mentioned it.
//
// A type parameter says the same thing directly, and the value-versus-pointer
// asymmetry that broke the template becomes an ordinary constructor argument.

// Predicate reports whether an object should be included in a listing.
type Predicate[T any] func(T) bool

// TypedLibrary is a Library narrowed to one element type.
type TypedLibrary[T any] interface {
	// Flush all active objects to disk
	Flush() error
	// Store the given object
	Store(object T) error
	// Id returns the primary identifier of the given object
	Id(object T) data.ID
	// Fetch the object with the given ID
	Fetch(id data.ID) (T, error)
	// Delete the object with the given ID
	Delete(id data.ID) error
	// DeleteObject deletes the given object
	DeleteObject(o T) error
	// List every object in the library
	List() <-chan T
	// ListMatching lists the objects satisfying the predicate
	ListMatching(pred Predicate[T]) <-chan T
	// PrintCache prints the cache, for debugging purposes
	PrintCache()
}

// The concrete libraries. These are aliases rather than named types so that
// callers keep using lib.KeyLibrary and friends unchanged.
type (
	KeyLibrary        = TypedLibrary[data.Key]
	AccountLibrary    = TypedLibrary[data.Account]
	ConnectionLibrary = TypedLibrary[connection.Connection]
	ChangeLibrary     = TypedLibrary[data.Change]
	PolicyLibrary     = TypedLibrary[data.KeyPolicy]

	KeyPredicate        = Predicate[data.Key]
	AccountPredicate    = Predicate[data.Account]
	ConnectionPredicate = Predicate[connection.Connection]
	ChangePredicate     = Predicate[data.Change]
	PolicyPredicate     = Predicate[data.KeyPolicy]
)

// coercion turns whatever the underlying library produced into the element
// type. It exists because objects read back off disk are pointers -- the
// library builds them with reflect.New -- while objects still in the cache from
// this run may be values.
type coercion[T any] func(interface{}) (T, bool)

// asInterface is the coercion for element types that are themselves
// interfaces, where a plain type assertion does the job.
func asInterface[T any](o interface{}) (T, bool) {
	v, ok := o.(T)
	return v, ok
}

// asChange is the coercion for data.Change, which is a value type and so needs
// both forms handled. This is the asymmetry that the sed template could not
// express.
func asChange(o interface{}) (data.Change, bool) {
	switch c := o.(type) {
	case *data.Change:
		return *c, true
	case data.Change:
		return c, true
	}
	return data.Change{}, false
}

// asPolicy is the same, for data.KeyPolicy.
func asPolicy(o interface{}) (data.KeyPolicy, bool) {
	switch p := o.(type) {
	case *data.KeyPolicy:
		return *p, true
	case data.KeyPolicy:
		return p, true
	}
	return data.KeyPolicy{}, false
}

type typedLibrary[T any] struct {
	Library
	// kind names the element type, for error messages only.
	kind   string
	coerce coercion[T]
}

func newTypedLibrary[T any](path, kind string, coerce coercion[T]) TypedLibrary[T] {
	return &typedLibrary[T]{
		Library: &library{Path: path},
		kind:    kind,
		coerce:  coerce,
	}
}

func NewKeyLibrary(path string) KeyLibrary {
	return newTypedLibrary(path, "Key", asInterface[data.Key])
}

func NewAccountLibrary(path string) AccountLibrary {
	return newTypedLibrary(path, "Account", asInterface[data.Account])
}

func NewConnectionLibrary(path string) ConnectionLibrary {
	return newTypedLibrary(path, "Connection", asInterface[connection.Connection])
}

func NewChangeLibrary(path string) ChangeLibrary {
	return newTypedLibrary(path, "Change", asChange)
}

func NewPolicyLibrary(path string) PolicyLibrary {
	return newTypedLibrary(path, "KeyPolicy", asPolicy)
}

func (l *typedLibrary[T]) Store(object T) error {
	return l.Library.Store(object)
}

func (l *typedLibrary[T]) Id(object T) data.ID {
	return data.ID(l.Library.Id(object))
}

func (l *typedLibrary[T]) Fetch(id data.ID) (T, error) {
	var zero T

	o, err := l.Library.Fetch(string(id))
	if err != nil {
		return zero, err
	}

	v, ok := l.coerce(o)
	if !ok {
		return zero, fmt.Errorf("ID %s was not a %s object", id, l.kind)
	}

	return v, nil
}

func (l *typedLibrary[T]) Delete(id data.ID) error {
	return l.Library.Delete(string(id))
}

func (l *typedLibrary[T]) DeleteObject(o T) error {
	return l.Library.DeleteObject(o)
}

func (l *typedLibrary[T]) List() <-chan T {
	return l.ListMatching(func(T) bool { return true })
}

func (l *typedLibrary[T]) ListMatching(predicate Predicate[T]) <-chan T {
	c := make(chan T)

	go func() {
		defer close(c)
		for o := range l.Library.List() {
			v, ok := l.coerce(o)
			if !ok {
				output.Error(fmt.Sprintf("while listing, object %v was not a %s", o, l.kind))
				continue
			}
			if predicate(v) {
				c <- v
			}
		}
	}()

	return c
}
