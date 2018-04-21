package data

import "sync"


type FanInKeys struct {
	wg sync.WaitGroup
	c chan Key
	doneAdding bool
}


func NewFanInKey() *FanInKeys {
	f :=  &FanInKeys{sync.WaitGroup{}, make(chan Key), false}
	return f
}

func (f *FanInKeys) Add(c chan Key) {
	f.wg.Add(1)
	go func() {
		for k := range c {
			f.c <- k
		}
		f.wg.Done()
	}()
}

func (f *FanInKeys) Output() chan Key {
	return f.c
}

func (f *FanInKeys) DoneAdding() {
	if !f.doneAdding {
		go func() {
			f.wg.Wait()
			close(f.c)
		}()
		f.doneAdding = true
	}
}

func (f *FanInKeys) Wait() {
	f.DoneAdding()
	f.wg.Wait()
}
