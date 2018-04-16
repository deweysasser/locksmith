package lib

import (
	"os"
	"io/ioutil"
	"fmt"
	"encoding/json"
)

type Storable interface {
	IdString() string
}

type library struct {
	Path string
	deserialize func(string, []byte) (Storable, error)
	cache map[string]Storable
}


type Library interface {
	// Flush all active objects to disk
	Flush() error
	// Store the given data as the ID
	store(object Storable) error
	// Fetch the data given by ID from the disk
	fetch(id string) (Storable, error)
	// Ensure that the object exists
	ensure(id string) (Storable, error)
	// Delete the object with the given ID from the disk
	delete(id string)
}


func (l *library) store(o Storable) error {
	_, e := os.Stat(l.Path)
	if e != nil {
		e = os.MkdirAll(l.Path, 777)
		if e != nil {
			return e
		}
	}

	path := fmt.Sprintf("%s/%s.json", l.Path, o.IdString())
	bytes, e := json.MarshalIndent(o, " ", " ")
	if e != nil {
		return e
	}
	e = ioutil.WriteFile(path, bytes , 666)
	return e
}

func (l *library) fetch(id string) (Storable, error) {
	path := fmt.Sprintf("%s/%s.json", l.Path, id)

	bytes, e := ioutil.ReadFile(path)
	if e != nil {
		return nil, e
	}

	o, e := l.deserialize(id, bytes)

	if e != nil {
		l.cache[id]=o
	}

	return o, e
}

func (l *library) Flush() error {
	for _, object := range(l.cache) {
		e := l.store(object)
		if e != nil {
			return e
		}
	}
	return nil
}

func (l *library) ensure(id string) (Storable, error) {
	o, e := l.fetch(id)
	if e == nil {
		return o, nil
	}

	o, e = l.deserialize(id, []byte("{}"))
	return o, e
}

func (l *library) delete(id string) error {
	path := fmt.Sprintf("%s/%s.json", l.Path, id)

	return os.Remove(path)
}