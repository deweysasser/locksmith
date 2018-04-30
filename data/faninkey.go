package data

import "sync"

type FanInKeys struct {
	c          chan Key
	wg         sync.WaitGroup
	doneAdding bool
}

func NewFanInKey(c chan Key) *FanInKeys {
	if c == nil {
		c = make(chan Key)
	}
	f := &FanInKeys{c: c, wg: sync.WaitGroup{}}
	return f
}

func (f *FanInKeys) Add(c <- chan Key) {
	f.wg.Add(1)
	go func() {
		for k := range c {
			f.c <- k
		}
		f.wg.Done()
	}()
}

func (f *FanInKeys) Input() chan Key {
	c := make(chan Key)
	f.Add(c)
	return c
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
