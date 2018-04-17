package lib

import (
	"os"
	"io/ioutil"
	"fmt"
	"encoding/json"
	"crypto/sha256"
)

type IdStringer interface {
	IdString() string
}

type Deserializer func(string, []byte) (interface{}, error)
type Ider func(interface{}) string

type library struct {
	Path string
	deserializer Deserializer
	ider Ider
	cache map[string]interface{}
}


type Library interface {
	// Flush all active objects to disk
	Flush() error
	// Store the given data as the ID
	Store(object interface{}) error
	// Fetch the data given by ID from the disk
	Fetch(id string) (interface{}, error)
	// Ensure that the object exists
	Ensure(id string) (interface{}, error)
	// Delete the object with the given ID from the disk
	Delete(id string) error
}

func (l *library) Init(path string, ider Ider, deserializer Deserializer) {
	l.Path = path
	l.ider = ider
	l.deserializer = deserializer
}

func (l *library) deserialize(id string, bytes []byte) (interface{}, error) {
	switch {
	case l.deserializer != nil:
		return l.deserializer(id, bytes)
	default:
		o := make(map[string]interface{})
		e := json.Unmarshal(bytes,&o)
		return o, e
	}
}

func (l *library) id(o interface{}) string {
	switch o.(type) {
	case IdStringer:
		return o.(IdStringer).IdString()
	case fmt.Stringer:
		return hashString(o.(fmt.Stringer).String())
	default:
		return hash(toJson(o))
	}
}


func hashString(s string) string {
	return hash([]byte(s))
}

func hash(s []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(s))
}

func toJson(o interface{}) []byte {
	bytes, e := json.MarshalIndent(o, " ", " ")
	//check(e)
	if e != nil {
		panic(e)
	}
	return bytes

}

func (l *library) Store(o interface{}) error {
	_, e := os.Stat(l.Path)
	if e != nil {
		e = os.MkdirAll(l.Path, 777)
		if e != nil {
			return e
		}
	}

	path := fmt.Sprintf("%s/%s.json", l.Path, l.id(o))
	bytes, e := json.MarshalIndent(o, " ", " ")
	if e != nil {
		return e
	}
	e = ioutil.WriteFile(path, bytes , 666)
	return e
}

func (l *library) Fetch(id string) (interface{}, error) {
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
		e := l.Store(object)
		if e != nil {
			return e
		}
	}
	return nil
}

func (l *library) Ensure(id string) (interface{}, error) {
	o, e := l.Fetch(id)
	if e == nil {
		return o, nil
	}

	o, e = l.deserialize(id, []byte("{}"))
	return o, e
}

func (l *library) Delete(id string) error {
	path := fmt.Sprintf("%s/%s.json", l.Path, id)

	return os.Remove(path)
}