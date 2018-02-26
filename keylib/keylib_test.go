package keylib

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
	os.RemoveAll("test-output/locksmith")

	lib := KeyLib{"test-output/locksmith"}

	key := keys.Read("../keys/test-data/rsa.pub")

	k, e := lib.Ingest(key)

	checke("Error ingesting key", t, e)

	if k == nil {
		t.Fatalf("Missing ingested key")
	}

	_, error := os.Stat("test-output/locksmith/keys/")

	checke("Did not create directory", t, error)

	lib2 := KeyLib{"test-output/locksmith"}

	keys, e := lib2.Keys()

	checke("Failed to list keys", t, e)

	if len(keys) != 1 {
		t.Fatalf(fmt.Sprintf("Expected 1 key, found %d keys", len(keys)))
	}

}
