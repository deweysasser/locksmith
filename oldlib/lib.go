package oldlib

import (
	"sync"
)

type library struct {
	Path string
}

type olib struct {
	library
	klib *KeyLib
	alib *Accountlib
	lock sync.Mutex
}

func NewLibrary(path string) *olib {
	return &olib{library{path}, nil, nil, sync.Mutex{}}
}

type ObjectLibrary interface {
	Keylib() *KeyLib
	Accountlib() *Accountlib
}

type Library interface {
	Save()
}


func (o *olib)Save() {
	o.lock.Lock()
	defer o.lock.Unlock()
	if o.alib != nil {
		// TODO
		//o.alib.Save()
	}
	if o.klib != nil {
		o.klib.Save()
	}
}

func (o *olib)Keylib() *KeyLib {
	o.lock.Lock()
	defer o.lock.Unlock()
	if o.klib == nil {
		o.klib = NewKeylib(o.Path)
	}
	return o.klib
}

func (o *olib)Accountlib() *Accountlib {
	o.lock.Lock()
	defer o.lock.Unlock()
	if o.alib == nil {
		o.alib = NewAccountlib(o.Path)
	}
	return o.alib
}
