package keylib

import (
	"fmt"
	"github.com/deweysasser/locksmith/keys"
	"io/ioutil"
	"os"
	"regexp"
)

type KeyLib struct {
	Path string
	keys []keys.Key
}

func New(path string) *KeyLib {
	return &KeyLib{path, nil}
}

func check(reason string, e error) {
	if e != nil {
		panic(fmt.Sprintf("%s: %s", reason, e))
	}
}

func (kl *KeyLib) keypath() string {
	keypath := kl.Path + "/keys"
	_, err := os.Stat(keypath)

	if err != nil {
		e := os.MkdirAll(keypath, 755)
		check("Failed to create dir", e)
	}

	return keypath
}

func (k *KeyLib) IngestFile(path string) (keys.Key, error) {
	key := keys.Read(path)
	return k.Ingest(key)
}

func (kl *KeyLib) Ingest(key keys.Key) (keys.Key, error) {
	re, err := regexp.Compile("[^a-zA-Z0-9]")
	check("Regexp failure", err)

	id := string(re.ReplaceAll([]byte(key.Id()), []byte("")))

	keyfile := kl.keypath() + "/" + id + ".json"

	json, error := key.Json()
	if error != nil {
		return nil, error
	}
	ioutil.WriteFile(keyfile, json, 0644)

	return key, nil
}

func (kl *KeyLib) Keys() ([]keys.Key, error) {
	if kl.keys != nil {
		return kl.keys, nil
	}
	
	keydir := kl.keypath()
	files, error := ioutil.ReadDir(keydir)
	fmt.Println("Reading", keydir)

	keylist := make([]keys.Key, 0)

	if error != nil {
		return nil, error
	}

	for _, path := range files {
		readpath := keydir + "/" + path.Name()
		keylist = append(keylist, keys.LoadJsonFile(readpath))
	}

	kl.keys = keylist
	
	return keylist, nil
}
