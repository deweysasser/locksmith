package data

import "sync"


type FanInAccounts struct {
	wg sync.WaitGroup
	c chan Account
}


func NewFanInAccount() *FanInAccounts {
	f :=  &FanInAccounts{sync.WaitGroup{}, make(chan Account)}
	return f
}

func (f *FanInAccounts) Output() chan Account {
	return f.c
}

func (f *FanInAccounts) Close() {
	go func () {
		f.wg.Wait()
		close(f.c)
	}()
}

func (f *FanInAccounts) Wait() {
	f.Close()
	f.wg.Wait()
}

func  (f *FanInAccounts) Input() chan Account {
	c := make(chan Account)
	f.wg.Add(1)

	go func() {
		for k := range c {
			f.c <- k
		}
		f.wg.Done()
	}()

	return c
}
