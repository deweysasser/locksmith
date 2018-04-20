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

func (f *FanInKeys) Output() chan Key {
	return f.c
}

func (f *FanInKeys) Close() {
	go func () {
		f.wg.Wait()
		close(f.c)
	}()
}

func (f *FanInKeys) Wait() {
	f.Close()
	f.wg.Wait()
}

func  (f *FanInKeys) Input() chan Key {
	c := make(chan Key)
	f.wg.Add(1)

	go func() {
		for k := range c {
			f.c <- k
		}
		f.wg.Done()
	}()

	return c
}
