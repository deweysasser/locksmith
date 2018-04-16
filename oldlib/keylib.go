package oldlib

import (
	"github.com/deweysasser/locksmith/data"
	"io/ioutil"
	"os"
	"sync"
	"regexp"
)

type KeyLib struct {
	library
	keys map[data.KeyID]data.Key
	lock sync.Mutex
}

func NewKeylib(path string) *KeyLib {
	return &KeyLib{library{path}, make(map[data.KeyID]data.Key), sync.Mutex{}}
}


func (kl *KeyLib) keypath() string {
	keypath := kl.Path + "/data"
	_, err := os.Stat(keypath)

	if err != nil {
		e := os.MkdirAll(keypath, 755)
		check("Failed to create dir", e)
	}

	return keypath
}

func (k *KeyLib) IngestFile(path string) (data.Key, error) {
	key := data.Read(path)
	return k.Ingest(key)
}

func (kl *KeyLib) Ingest(key data.Key) (data.Key, error) {
	kl.keys[key.Id()]=key
	return key, nil
}

func (kl *KeyLib) Save() {
	for keyid, key:= range(kl.keys) {
		re, err := regexp.Compile("[^a-zA-Z0-9]")
		check("Regexp failure", err)
		id := string(re.ReplaceAll([]byte(keyid), []byte("")))
		keyfile := kl.keypath() + "/" + id + ".json"
		saveKey(key, keyfile)
	}
}

func saveKey(k data.Key, keyfile string) (data.Key, error) {
	json, error := k.Json()
	if error != nil {
		return nil, error
	}
	ioutil.WriteFile(keyfile, json, 0644)

	return k, nil
}

func (kl *KeyLib) AllKeys() (chan data.Key, error) {
	c:= make(chan data.Key)
	
	keydir := kl.keypath()
	files, error := ioutil.ReadDir(keydir)
//	fmt.Println("Reading", keydir)

	if error != nil {
		return nil, error
	}

	go func() {
		for _, path := range files {
			readpath := keydir + "/" + path.Name()
			c <- data.LoadJsonFile(readpath)
		}
		close(c)
	}()
	
	return c, nil
}
