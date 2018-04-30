package lib

import (
	"encoding/json"
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"os"
	"reflect"
	"testing"
	"time"
)

type entry struct {
	Id, Content string
}

func (e *entry) IdString() string {
	return e.Id
}

func createEntry(id string, bytes []byte) (interface{}, error) {
	e := new(entry)
	err := json.Unmarshal(bytes, &e)

	e.Id = id

	return e, err
}

var testDirectory string = "test-output/lib-test"

func TestMain(m *testing.M) {
	os.RemoveAll(testDirectory)
	os.Exit(m.Run())
}

func check(t *testing.T, e error, s string) {
	if e != nil {
		t.Error("Unexpected error during '" + s + "': " + e.Error())
	}
}

func checkStringEquals(t *testing.T, message, s1, s2 string) {
	if s1 != s2 {
		t.Error(message)
	}
}

func TestLibrary(t *testing.T) {
	lib := new(library)
	lib.Path = testDirectory
	lib.deserializer = createEntry

	e := entry{"id1", "testing1"}

	lib.Store(&e)

	_, err := os.Stat(testDirectory + "/id1.json")
	check(t, err, "File does not exist")

	e2, err := lib.Fetch("id1")

	check(t, err, "Failed to restore")

	checkStringEquals(t, "Failed to restore correct content", e.Content, e2.(*entry).Content)
}

func TestSave(t *testing.T) {
	lib := new(library)
	lib.Init(testDirectory, nil, createEntry)

	e := entry{"id1", "testing1"}

	lib.Store(&e)

	_, err := os.Stat(testDirectory + "/id1.json")
	check(t, err, "File does not exist")

	lib2 := new(library)
	lib2.Init(testDirectory, nil, createEntry)

	e2, err := lib2.Fetch("id1")

	check(t, err, "Failed to restore")

	checkStringEquals(t, "Failed to restore correct content", e.Content, e2.(*entry).Content)

	_, err2 := lib.Fetch("id2")
	if err2 == nil {
		t.Error("Should have failed to find")
	}

	s, err2 := lib.Ensure("id2")

	check(t, err2, "Ensure errored")

	if s.(*entry).IdString() != "id2" {
		t.Error("Wrong ID: " + s.(*entry).IdString())
	}

	if s.(*entry).Content != "" {
		t.Error("Should not have value")
	}

	_, err3 := os.Stat(testDirectory + "/id2.json")
	if err3 == nil {
		t.Error("File should not exist")
	}

	err4 := lib2.Flush()

	check(t, err4, "Flush Failed")

	_, err5 := os.Stat(testDirectory + "/id2.json")
	if err5 == nil {
		t.Error("File should not exist")
	}

}

func assertStringEquals(t *testing.T, message, s1, s2 string) {
	if s1 != s2 {
		t.Error(message)
	}
}

func TestAWSKey(t *testing.T) {
	os.RemoveAll("test-output")

	ml := MainLibrary{Path: "test-output"}

	lib := ml.Keys()

	k := data.NewAwsKey("testing",  "test", time.Now())

	assertStringEquals(t, "ID of aws key is wrong", lib.(*library).Id(k), "testing")

	lib.Store(k)

	_, err := os.Stat("test-output/keys/testing.json")

	check(t, err, "Writing correct json file")

	lib.Flush()

	i2, err := lib.Fetch("testing")

	check(t, err, "Failed to fetch key")

	if k.Id() != i2.(data.Key).Id() {
		t.Error("Keys did not match")
	}

}

type withID struct {
	Type string
	ID   string
}

func (t *withID) IdString() string {
	return t.ID
}

type Type1 struct {
	withID
	Name1 string
}

type Type2 struct {
	withID
	Name2 string
}

func TestIdConversion(t *testing.T) {
	lib := library{Path: "test-output/type-test"}
	var t1 = &Type1{withID{"Type1", "id1"}, "name1"}

	assertStringEquals(t, "Verify ID", "id1", lib.Id(t1))
}

func init() {
	AddType(reflect.TypeOf(Type1{}))
	AddType(reflect.TypeOf(Type2{}))
}

func TestReflectionDeserialize(t *testing.T) {

	lib := library{Path: "test-output/type-test"}

	t1 := Type1{withID{"Type1", "id1"}, "name1"}
	t2 := Type2{withID{"Type2", "id2"}, "name2"}

	lib.Store(&t1)
	lib.Store(&t2)

	t1a, err := lib.Fetch("id1")

	check(t, err, "Error fetching id1")

	if _, ok := t1a.(Type1); ok {
		t.Error("Restored wrong type")
	}

	assertStringEquals(t, "Failed to restore id1", "name1", t1a.(*Type1).Name1)
}

type multiID struct {
	Ids []data.ID
}

func deserializeMultiID(id string, bytes []byte) (interface{}, error) {
	i := new(multiID)
	return i, json.Unmarshal(bytes, i)
}

func (m *multiID) Id() data.ID {
	return m.Ids[0]
}

func (m *multiID) Identifiers() []data.ID {
	return m.Ids
}

func TestMultipleIds(t *testing.T) {
	lib := library{Path: "test-output/test-multiple-ids", deserializer: deserializeMultiID}

	m := multiID{[]data.ID{data.ID("id1"), data.ID("id2")}}

	lib.Store(&m)

	t.Run("Through stored lib", func(t *testing.T) {
		verifyMultiIDs(lib, m, t)
	})

	lib2 := library{Path: "test-output/test-multiple-ids", deserializer: deserializeMultiID}

	t.Run("Through second lib", func(t *testing.T) {
		verifyMultiIDs(lib2, m, t)
	})
}

func verifyMultiIDs(lib library, m multiID, t *testing.T) {
	if m2, e := lib.Fetch("id1"); e == nil {
		m2a := m2.(*multiID)
		if fmt.Sprintf("%s", m.Ids) == fmt.Sprintf("%s", m2a) {
			t.Error("Failed to look up by primary ID", m.Ids, m2a.Ids)
		}
	} else {
		t.Error("Failed to fetch by primary ID", e)
	}

	fmt.Println(lib.cache)

	if m2, e := lib.Fetch("id2"); e == nil {
		m2a := m2.(*multiID)
		if fmt.Sprintf("%s", m.Ids) == fmt.Sprintf("%s", m2a) {
			t.Error("Failed to look up by secondary ID", m.Ids, m2a.Ids)
		}
	} else {
		t.Error("Failed to fetch by seondary ID", e)
	}
}
