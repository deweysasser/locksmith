package lib

//GENERATED FILE

import (
	"errors"
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

type changeLibrary struct {
	Library
}

type ChangePredicate func(key data.Change) bool

type ChangeLibrary interface {
	// Flush all active objects to disk
	Flush() error
	// Store the given data as the ID
	Store(object data.Change) error
	// Get the ID of the given object
	Id(object data.Change) data.ID
	// Fetch the data given by ID from the disk
	Fetch(id data.ID) (data.Change, error)
	// Delete the object with the given ID from the disk
	Delete(id data.ID) error
	// Delete the object given
	DeleteObject(o data.Change) error
	// List the objects
	List() <-chan data.Change
	// List the objects that match the given predicate
	ListMatching(pred ChangePredicate) <-chan data.Change
	// List the objects to a generic channel
	ListGeneric() <-chan interface{}
	// Print the cache, for debugging purposes
	PrintCache()
}

func NewChangeLibrary(path string) ChangeLibrary {
	l := new(changeLibrary)
	l.Library = &library{Path: path}

	return l
}

func (l *changeLibrary) Store(object data.Change) error {
	return l.Library.Store(object)
}

func (l *changeLibrary) Id(object data.Change) data.ID {
	return data.ID(l.Library.Id(object))
}
func (l *changeLibrary) Fetch(id data.ID) (data.Change, error) {
	if o, err := l.Library.Fetch(string(id)); err == nil {
		if k, ok := o.(data.Change); ok {
			return k, nil
		} else {
			return data.Change{}, errors.New(fmt.Sprint("ID ", id, " was not a Change object"))
		}
	} else {
		return data.Change{}, err
	}
}
func (l *changeLibrary) Delete(id data.ID) error {
	return l.Library.Delete(string(id))
}
func (l *changeLibrary) DeleteObject(o data.Change) error {
	return l.Library.DeleteObject(o)
}

func (l *changeLibrary) List() <-chan data.Change {
	return l.ListMatching(func(d data.Change) bool { return true })
}

func (l *changeLibrary) ListMatching(predicate ChangePredicate) <-chan data.Change {
	c := make(chan data.Change)
	go func() {
		defer close(c)
		for o := range l.Library.List() {
			if k, ok := o.(*data.Change); ok {
				if predicate(*k) {
					c <- *k
				} else {
					output.Error(fmt.Sprint("while listing keys, object ", o, " was not a Change"))
				}
			} else {
				output.Error("Object recovered from", o, " is not a change")
			}
		}
	}()

	return c
}

func (l *changeLibrary) ListGeneric() <-chan interface{} {
	d := make(chan interface{})
	go func() {
		for o := range l.List() {
			d <- o
		}
	}()

	return d
}
