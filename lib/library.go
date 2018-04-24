package lib

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"reflect"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
)

var TypeMap = make(map[string]reflect.Type)

func AddType(p reflect.Type) {
	TypeMap[p.Name()] = p
}

type IdStringer interface {
	IdString() string
}

type Deserializer func(string, []byte) (interface{}, error)
type IdFunction func(interface{}) string

type library struct {
	Path         string
	deserializer Deserializer
	idfunc       IdFunction
	cache 		map[string]interface{}
	cacheLoaded bool
	// If we want to make store *NOT* hit disk, then uncomment and implement
	//changes map[string]interface{}
}

type Library interface {
	// Flush all active objects to disk
	Flush() error
	// Store the given data as the ID
	Store(object interface{}) error
	// Get the ID of the given object
	Id(object interface{}) string
	// Fetch the data given by ID from the disk
	Fetch(id string) (interface{}, error)
	// Ensure that the object exists
	Ensure(id string) (interface{}, error)
	// Delete the object with the given ID from the disk
	Delete(id string) error
	// List the objects
	List() chan interface{}
}

func (l *library) Init(path string, idfunc IdFunction, deserializer Deserializer) {
	l.Path = path
	l.idfunc = idfunc
	l.deserializer = deserializer
}

func (l *library) deserialize(id string, bytes []byte) (interface{}, error) {
	switch {
	case l.deserializer != nil:
		return l.deserializer(id, bytes)
	default:
		o := make(map[string]interface{})
		e := json.Unmarshal(bytes, &o)
		if t, ok := o["Type"]; ok {
			if strT, ok := t.(string); ok { // it's a string
				if p, ok := TypeMap[strT]; ok { // it's in the type map
					no := reflect.New(p).Interface()
					e := json.Unmarshal(bytes, &no)
					return no, e
				}
			}
		}
		return o, e
	}
}

/* Return the primary identifier for this object
 */
func (l *library) Id(o interface{}) string {
	//fmt.Printf("type is %s\n", reflect.TypeOf(o))

	if i, ok := o.(data.Ider); ok {
		return string(i.Id())
	}

	if i, ok := o.(IdStringer); ok {
		return i.IdString()
	}

	if s, ok := o.(fmt.Stringer); ok {
		return hashString(s.String())
	}
	return hash(toJson(o))
}

/** Return the set of identifiers used by this object.  Each identifier must be unique to this object, but there (obviously) can be many.
 */
func (l *library) ids(o interface{}) chan string {
	c:= make(chan string)
	go func() {
		defer close(c)
		if i, ok := o.(data.Identiferser); ok {
			for _, id:= range i.Identifers() {
				c <- string(id)
			}
		} else {
			c <- l.Id(o)
		}
	}()
	return c
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

	primaryID := l.Id(o)
	path := fmt.Sprintf("%s/%s.json", l.Path, sanitize(primaryID))
	//fmt.Println("Writing to " , path)
	bytes, e := json.MarshalIndent(o, " ", " ")
	if e != nil {
		return e
	}
	e = ioutil.WriteFile(path, bytes, 666)

	if e == nil {
		l.addToCache(o)
	}
	return e
}

func (l *library) addToCache(o interface{}) {
	if l.cache == nil {
		l.cache = make(map[string]interface{})
	}
	for id := range l.ids(o) {
		l.cache[id] = o
	}
}

func (l *library) Fetch(id string) (interface{}, error) {
	l.Load()
	if l.cache != nil {
		if v, ok := l.cache[id]; ok {
			return v, nil
		}
	}
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
		//l.cache[Id] = o
	} else {
		output.Error("Failed to read key in " + path)
	}

	//fmt.Printf("Read %s\n", o)
	return o, e
}

func (l *library) Flush() error {
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

func (l *library) Load()  {
	if l.cache == nil {
		l.cache = make(map[string]interface{})
	}

	if !l.cacheLoaded {
		l.cacheLoaded = true
		for o := range l.List() {
			l.addToCache(o)
		}
	}
}

func (l *library) List() (c chan interface{}) {
	//fmt.Println("Fetching connections from ", lib.Path)
	c = make(chan interface{})

	_, e := os.Stat(l.Path)

	if e != nil {
		close(c)
		return
	}

	files, e := ioutil.ReadDir(l.Path)

	//fmt.Println("Reading files in ", lib.Path)

	if e != nil {
		close(c)
		return
	}

	go readFiles(l, files, c)
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
