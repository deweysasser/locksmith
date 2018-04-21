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

func (f *FanInAccounts) Add(c chan Account) {
	f.wg.Add(1)
	go func() {
		for k := range c {
			f.c <- k
		}
		f.wg.Done()
	}()
}

func (f *FanInAccounts) Output() chan Account {
	return f.c
}

func (f *FanInAccounts) Wait() {
	f.wg.Wait()
	f.Close()
}

func (f *FanInAccounts) Close() {
	close(f.c)
}