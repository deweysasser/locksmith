package oldlib

import (
	"fmt"
	"github.com/deweysasser/locksmith/data"
	"os"
	"testing"
)

func checke(message string, t *testing.T, e error) {
	if e != nil {
		t.Fatalf(fmt.Sprintf("%s: %s", message, e))
	}
}

func TestKeyIngest(t *testing.T) {
	os.RemoveAll("test-output/data")

	lib := NewKeylib("test-output/data")

	key := data.Read("../data/test-data/rsa.pub")

	k, e := lib.Ingest(key)

	checke("Error ingesting key", t, e)

	if k == nil {
		t.Fatalf("Missing ingested key")
	}

	// TODO:  this should return an error code
	lib.Save()

	//	checke("Error saving data", t, e)

	_, error := os.Stat("test-output/data/")

	checke("Did not create directory", t, error)

	lib2 := NewKeylib("test-output/data")

	keys, e := lib2.AllKeys()

	checke("Failed to list data", t, e)

	var count int
	for k := range keys {
		if &k != nil {
			count++
		}
	}

	if count != 1 {
		t.Fatalf(fmt.Sprintf("Expected 1 key, found %d data", len(keys)))
	}

}

