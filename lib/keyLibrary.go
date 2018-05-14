package lib

//go:generate make

import (
	"errors"
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

type keyLibrary struct {
	Library
}

type KeyPredicate func(key data.Key) bool

type KeyLibrary interface {
	// Flush all active objects to disk
	Flush() error
	// Store the given data as the ID
	Store(object data.Key) error
	// Get the ID of the given object
	Id(object data.Key) data.ID
	// Fetch the data given by ID from the disk
	Fetch(id data.ID) (data.Key, error)
	// Delete the object with the given ID from the disk
	Delete(id data.ID) error
	// Delete the object given
	DeleteObject(o data.Key) error
	// List the objects
	List() <-chan data.Key
	// List the objects that match the given predicate
	ListMatching(pred KeyPredicate) <-chan data.Key
	// List the objects to a generic channel
	ListGeneric() <-chan interface{}
	// Print the cache, for debugging purposes
	PrintCache()
}

func NewKeyLibrary(path string) KeyLibrary {
	l := new(keyLibrary)
	l.Library = &library{Path: path}

	return l
}

func (l *keyLibrary) Store(object data.Key) error {
	return l.Library.Store(object)
}

func (l *keyLibrary) Id(object data.Key) data.ID {
	return data.ID(l.Library.Id(object))
}
func (l *keyLibrary) Fetch(id data.ID) (data.Key, error) {
	if o, err := l.Library.Fetch(string(id)); err == nil {
		if k, ok := o.(data.Key); ok {
			return k, nil
		} else {
			return nil, errors.New(fmt.Sprint("ID ", id, " was not a Key object"))
		}
	} else {
		return nil, err
	}
}
func (l *keyLibrary) Delete(id data.ID) error {
	return l.Library.Delete(string(id))
}
func (l *keyLibrary) DeleteObject(o data.Key) error {
	return l.Library.DeleteObject(o)
}

func (l *keyLibrary) List() <-chan data.Key {
	return l.ListMatching(func(d data.Key) bool { return true })
}

func (l *keyLibrary) ListMatching(predicate KeyPredicate) <-chan data.Key {
	c := make(chan data.Key)
	go func() {
		defer close(c)
		for o := range l.Library.List() {
			if k, ok := o.(data.Key); ok {
				if predicate(k) {
					c <- k
				}
			} else {
				output.Error(fmt.Sprint("while listing, object ", o, " was not a Key"))
			}
		}
	}()

	return c
}

func (l *keyLibrary) ListGeneric() <-chan interface{} {
	d := make(chan interface{})
	go func() {
		for o := range l.List() {
			d <- o
		}
	}()

	return d
}
