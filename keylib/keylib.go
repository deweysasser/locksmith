package keylib

import "fmt"
import "github.com/deweysasser/locksmith/keys"

type KeyLib struct {
	Path string
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func (k *KeyLib) IngestFile(path string) keys.Key {
	key := keys.Read(path)
	fmt.Printf("Ingested %s", key)
	return key
}
