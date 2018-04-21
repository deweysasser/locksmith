package data

import "sync"


type FanInKeys struct {
	wg sync.WaitGroup
	c chan Key
}


func NewFanInKey() *FanInKeys {
	f :=  &FanInKeys{sync.WaitGroup{}, make(chan Key)}
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

func (f *FanInKeys) Wait() {
	f.wg.Wait()
	f.Close()
}

func (f *FanInKeys) Close() {
	close(f.c)
}