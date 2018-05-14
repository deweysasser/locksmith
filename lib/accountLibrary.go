package lib

//GENERATED FILE

import (
	"errors"
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

type accountLibrary struct {
	Library
}

type AccountPredicate func(key data.Account) bool

type AccountLibrary interface {
	// Flush all active objects to disk
	Flush() error
	// Store the given data as the ID
	Store(object data.Account) error
	// Get the ID of the given object
	Id(object data.Account) data.ID
	// Fetch the data given by ID from the disk
	Fetch(id data.ID) (data.Account, error)
	// Delete the object with the given ID from the disk
	Delete(id data.ID) error
	// Delete the object given
	DeleteObject(o data.Account) error
	// List the objects
	List() <-chan data.Account
	// List the objects that match the given predicate
	ListMatching(pred AccountPredicate) <-chan data.Account
	// List the objects to a generic channel
	ListGeneric() <-chan interface{}
	// Print the cache, for debugging purposes
	PrintCache()
}

func NewAccountLibrary(path string) AccountLibrary {
	l := new(accountLibrary)
	l.Library = &library{Path: path}

	return l
}

func (l *accountLibrary) Store(object data.Account) error {
	return l.Library.Store(object)
}

func (l *accountLibrary) Id(object data.Account) data.ID {
	return data.ID(l.Library.Id(object))
}
func (l *accountLibrary) Fetch(id data.ID) (data.Account, error) {
	if o, err := l.Library.Fetch(string(id)); err == nil {
		if k, ok := o.(data.Account); ok {
			return k, nil
		} else {
			return nil, errors.New(fmt.Sprint("ID ", id, " was not a Account object"))
		}
	} else {
		return nil, err
	}
}
func (l *accountLibrary) Delete(id data.ID) error {
	return l.Library.Delete(string(id))
}
func (l *accountLibrary) DeleteObject(o data.Account) error {
	return l.Library.DeleteObject(o)
}

func (l *accountLibrary) List() <-chan data.Account {
	return l.ListMatching(func(d data.Account) bool { return true })
}

func (l *accountLibrary) ListMatching(predicate AccountPredicate) <-chan data.Account {
	c := make(chan data.Account)
	go func() {
		defer close(c)
		for o := range l.Library.List() {
			if k, ok := o.(data.Account); ok {
				if predicate(k) {
					c <- k
				}
			} else {
				output.Error(fmt.Sprint("while listing, object ", o, " was not a Account"))
			}
		}
	}()

	return c
}

func (l *accountLibrary) ListGeneric() <-chan interface{} {
	d := make(chan interface{})
	go func() {
		for o := range l.List() {
			d <- o
		}
	}()

	return d
}
