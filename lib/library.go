package lib

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
)

type IdStringer interface {
	IdString() string
}

type Deserializer func(string, []byte) (interface{}, error)
type Ider func(interface{}) string

type library struct {
	Path         string
	deserializer Deserializer
	ider         Ider
	cache        map[string]interface{}
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
	// List the objects
	List() chan interface{}
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
		e := json.Unmarshal(bytes, &o)
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

	path := fmt.Sprintf("%s/%s.json", l.Path, sanitize(l.id(o)))
	//fmt.Println("Writing to " , path)
	bytes, e := json.MarshalIndent(o, " ", " ")
	if e != nil {
		return e
	}
	e = ioutil.WriteFile(path, bytes, 666)
	return e
}

func (l *library) Fetch(id string) (interface{}, error) {
	path := fmt.Sprintf("%s/%s.json", l.Path, sanitize(id))
	return l.fetchFrom(id, path)
}

func sanitize(path string) string {
re := regexp.MustCompile(`\W+`)
return 	re.ReplaceAllString(path, "")

}

func (l *library) fetchFrom(id, path string) (interface{}, error) {

	bytes, e := ioutil.ReadFile(path)
	if e != nil {
		return nil, e
	}

	o, e := l.deserialize(id, bytes)

	if e == nil {
		//l.cache[id] = o
	} else {
		fmt.Println("Failed to read key in " + path)
	}

	//fmt.Printf("Read %s\n", o)
	return o, e
}

func (l *library) Flush() error {
	for _, object := range l.cache {
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

func (lib *library) List() (c chan interface{}) {
	//fmt.Println("Fetching connections from ", lib.Path)
	c = make(chan interface{})

	_, error := os.Stat(lib.Path)

	if error != nil {
		close(c)
		return
	}

	files, error := ioutil.ReadDir(lib.Path)

	//fmt.Println("Reading files in ", lib.Path)

	if error != nil {
		close(c)
		return
	}

	go readFiles(lib, files, c)
	return
}

func readFiles(lib *library, files []os.FileInfo, c chan interface{}) {
	defer close(c)

	for _, f := range files {
		path := lib.Path + "/" + f.Name()
		//fmt.Println("Reading from ", path)
		o, e := lib.fetchFrom("", path)

		if e == nil {
			//fmt.Println("Enqueuing ", o)
			c <- o
		}

	}
}
