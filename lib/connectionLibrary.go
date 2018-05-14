package lib

//GENERATED FILE

import (
	"errors"
	"fmt"
	"github.com/deweysasser/locksmith/connection"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

type connectionLibrary struct {
	Library
}

type ConnectionPredicate func(key connection.Connection) bool

type ConnectionLibrary interface {
	// Flush all active objects to disk
	Flush() error
	// Store the given data as the ID
	Store(object connection.Connection) error
	// Get the ID of the given object
	Id(object connection.Connection) data.ID
	// Fetch the data given by ID from the disk
	Fetch(id data.ID) (connection.Connection, error)
	// Delete the object with the given ID from the disk
	Delete(id data.ID) error
	// Delete the object given
	DeleteObject(o connection.Connection) error
	// List the objects
	List() <-chan connection.Connection
	// List the objects that match the given predicate
	ListMatching(pred ConnectionPredicate) <-chan connection.Connection
	// List the objects to a generic channel
	ListGeneric() <-chan interface{}
	// Print the cache, for debugging purposes
	PrintCache()
}

func NewConnectionLibrary(path string) ConnectionLibrary {
	l := new(connectionLibrary)
	l.Library = &library{Path: path}

	return l
}

func (l *connectionLibrary) Store(object connection.Connection) error {
	return l.Library.Store(object)
}

func (l *connectionLibrary) Id(object connection.Connection) data.ID {
	return data.ID(l.Library.Id(object))
}
func (l *connectionLibrary) Fetch(id data.ID) (connection.Connection, error) {
	if o, err := l.Library.Fetch(string(id)); err == nil {
		if k, ok := o.(connection.Connection); ok {
			return k, nil
		} else {
			return nil, errors.New(fmt.Sprint("ID ", id, " was not a Connection object"))
		}
	} else {
		return nil, err
	}
}
func (l *connectionLibrary) Delete(id data.ID) error {
	return l.Library.Delete(string(id))
}
func (l *connectionLibrary) DeleteObject(o connection.Connection) error {
	return l.Library.DeleteObject(o)
}

func (l *connectionLibrary) List() <-chan connection.Connection {
	return l.ListMatching(func(d connection.Connection) bool { return true })
}

func (l *connectionLibrary) ListMatching(predicate ConnectionPredicate) <-chan connection.Connection {
	c := make(chan connection.Connection)
	go func() {
		defer close(c)
		for o := range l.Library.List() {
			if k, ok := o.(connection.Connection); ok {
				if predicate(k) {
					c <- k
				} else {
					output.Error(fmt.Sprint("while listing keys, object ", o, " was not a Connection"))
				}
			}
		}
	}()

	return c
}

func (l *connectionLibrary) ListGeneric() <-chan interface{} {
	d := make(chan interface{})
	go func() {
		for o := range l.List() {
			d <- o
		}
	}()

	return d
}
