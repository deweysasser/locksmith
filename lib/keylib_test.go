package lib

import (
	"fmt"
	"github.com/deweysasser/locksmith/keys"
	"os"
	"testing"
)

func checke(message string, t *testing.T, e error) {
	if e != nil {
		t.Fatalf(fmt.Sprintf("%s: %s", message, e))
	}
}

func TestKeyIngest(t *testing.T) {
	os.RemoveAll("test-output/keys")

	lib := NewKeylib("test-output/keys")

	key := keys.Read("../keys/test-data/rsa.pub")

	k, e := lib.Ingest(key)

	checke("Error ingesting key", t, e)

	if k == nil {
		t.Fatalf("Missing ingested key")
	}

	// TODO:  this should return an error code
	lib.Save()

//	checke("Error saving keys", t, e)

	_, error := os.Stat("test-output/keys/")

	checke("Did not create directory", t, error)

	lib2 := NewKeylib("test-output/keys")

	keys, e := lib2.AllKeys()

	checke("Failed to list keys", t, e)

	var count int
	for k := range(keys) {
		if &k != nil{
			count++
		}
	}

	if count != 1 {
		t.Fatalf(fmt.Sprintf("Expected 1 key, found %d keys", len(keys)))
	}

}
