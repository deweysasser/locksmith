package data

import "sync"


type FanInAccounts struct {
	wg sync.WaitGroup
	c chan Account
	doneAdding bool
}


func NewFanInAccount() *FanInAccounts {
	f :=  &FanInAccounts{sync.WaitGroup{}, make(chan Account), false}
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

func (f *FanInAccounts) DoneAdding() {
	if !f.doneAdding {
		go func() {
			f.wg.Wait()
			close(f.c)
		}()
		f.doneAdding = true
	}
}

func (f *FanInAccounts) Wait() {
	f.DoneAdding()
	f.wg.Wait()
}
