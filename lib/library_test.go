package lib

import (
	"encoding/json"
	"os"
	"testing"
	"github.com/deweysasser/locksmith/data"
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

var testdir = "test-output/lib-test"

func TestMain(m *testing.M) {
	os.RemoveAll(testdir)
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
	lib.Path = testdir
	lib.deserializer = createEntry

	e := entry{"id1", "testing1"}

	lib.Store(&e)

	_, err := os.Stat(testdir + "/id1.json")
	check(t, err, "File does not exist")

	e2, err := lib.Fetch("id1")

	check(t, err, "Failed to restore")

	checkStringEquals(t, "Failed to restore correct content", e.Content, e2.(*entry).Content)
}

func TestSave(t *testing.T) {
	lib := new(library)
	lib.Init(testdir, nil, createEntry)

	e := entry{"id1", "testing1"}

	lib.Store(&e)

	_, err := os.Stat(testdir + "/id1.json")
	check(t, err, "File does not exist")

	lib2 := new(library)
	lib2.Init(testdir, nil, createEntry)

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

	_, err3 := os.Stat(testdir + "/id2.json")
	if err3 == nil {
		t.Error("File should not exist")
	}

	err4 := lib2.Flush()

	check(t, err4, "Flush Failed")

	_, err5 := os.Stat(testdir + "/id2.json")
	if err5 == nil {
		t.Error("File should not exist")
	}

}


func assertStringEquals(t *testing.T, message, s1, s2 string) {
	if s1 != s1 {
		t.Error(message)
	}
}


func TestAWSKey(t *testing.T) {
	os.RemoveAll("test-output")

	ml := MainLibrary{ Path:"test-output"}

	lib := ml.Keys()

	k := data.NewAwsKey("testing", "test")

	assertStringEquals(t, "ID of aws key is wrong", lib.(*library).id(k), "testing")

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