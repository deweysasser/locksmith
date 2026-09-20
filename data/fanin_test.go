package data

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// drainKeys reads the fan-in output until it closes, failing the test if that
// takes suspiciously long (a fan-in that never closes would otherwise hang the
// whole suite).
func drainKeys(t *testing.T, f *FanInKeys) []Key {
	t.Helper()

	done := make(chan []Key)
	go func() {
		var got []Key
		for k := range f.Output() {
			got = append(got, k)
		}
		done <- got
	}()

	select {
	case got := <-done:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("fan-in output channel was never closed")
		return nil
	}
}

func keySource(ids ...string) chan Key {
	c := make(chan Key)
	go func() {
		defer close(c)
		for _, id := range ids {
			c <- NewAwsKey(id, time.Time{}, true, id)
		}
	}()
	return c
}

func TestFanInKeysMultipleSources(t *testing.T) {
	f := NewFanInKey(nil)
	f.Add(keySource("AKIA1", "AKIA2"))
	f.Add(keySource("AKIA3"))
	f.DoneAdding()

	got := drainKeys(t, f)
	if len(got) != 3 {
		t.Errorf("got %d keys, want 3", len(got))
	}
}

func TestFanInKeysNoSources(t *testing.T) {
	f := NewFanInKey(nil)
	f.DoneAdding()

	if got := drainKeys(t, f); len(got) != 0 {
		t.Errorf("got %d keys from a fan-in with no sources, want 0", len(got))
	}
}

func TestFanInKeysEmptySource(t *testing.T) {
	f := NewFanInKey(nil)
	empty := make(chan Key)
	close(empty)
	f.Add(empty)
	f.DoneAdding()

	if got := drainKeys(t, f); len(got) != 0 {
		t.Errorf("got %d keys from an empty source, want 0", len(got))
	}
}

// DoneAdding is called both directly and via Wait, so calling it more than once
// must not close the output channel twice.
func TestFanInKeysDoneAddingIsIdempotent(t *testing.T) {
	f := NewFanInKey(nil)
	f.Add(keySource("AKIA1"))
	f.DoneAdding()
	f.DoneAdding()
	f.DoneAdding()

	if got := drainKeys(t, f); len(got) != 1 {
		t.Errorf("got %d keys, want 1", len(got))
	}
}

func TestFanInKeysUsesSuppliedChannel(t *testing.T) {
	mine := make(chan Key)
	f := NewFanInKey(mine)

	if f.Output() != mine {
		t.Error("NewFanInKey should fan in to the channel it was given")
	}
}

// Input hands back a channel already registered as a source.
func TestFanInKeysInput(t *testing.T) {
	f := NewFanInKey(nil)
	in := f.Input()

	go func() {
		defer close(in)
		in <- NewAwsKey("AKIA1", time.Time{}, true, "one")
	}()
	f.DoneAdding()

	if got := drainKeys(t, f); len(got) != 1 {
		t.Errorf("got %d keys, want 1", len(got))
	}
}

func TestFanInKeysWaitReturnsAfterAllSourcesDrain(t *testing.T) {
	f := NewFanInKey(nil)
	f.Add(keySource("AKIA1", "AKIA2"))

	// Wait blocks until every source goroutine has handed off its keys, so a
	// reader has to be running.
	var got []Key
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := range f.Output() {
			got = append(got, k)
		}
	}()

	f.Wait()
	wg.Wait()

	if len(got) != 2 {
		t.Errorf("got %d keys, want 2", len(got))
	}
}

func accountSource(names ...string) chan Account {
	c := make(chan Account)
	go func() {
		defer close(c)
		for _, n := range names {
			c <- NewSSHAccount("root", n, "conn1", nil)
		}
	}()
	return c
}

func drainAccounts(t *testing.T, f *FanInAccounts) []Account {
	t.Helper()

	done := make(chan []Account)
	go func() {
		var got []Account
		for a := range f.Output() {
			got = append(got, a)
		}
		done <- got
	}()

	select {
	case got := <-done:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("fan-in output channel was never closed")
		return nil
	}
}

func TestFanInAccountsMultipleSources(t *testing.T) {
	f := NewFanInAccount()
	f.Add(accountSource("a.example.com", "b.example.com"))
	f.Add(accountSource("c.example.com"))
	f.DoneAdding()

	if got := drainAccounts(t, f); len(got) != 3 {
		t.Errorf("got %d accounts, want 3", len(got))
	}
}

func TestFanInAccountsNoSources(t *testing.T) {
	f := NewFanInAccount()
	f.DoneAdding()

	if got := drainAccounts(t, f); len(got) != 0 {
		t.Errorf("got %d accounts, want 0", len(got))
	}
}

func TestFanInAccountsDoneAddingIsIdempotent(t *testing.T) {
	f := NewFanInAccount()
	f.Add(accountSource("a.example.com"))
	f.DoneAdding()
	f.DoneAdding()

	if got := drainAccounts(t, f); len(got) != 1 {
		t.Errorf("got %d accounts, want 1", len(got))
	}
}

// Fetching runs one connection per goroutine, so the fan-in has to survive
// many sources delivering at once.
func TestFanInKeysUnderConcurrency(t *testing.T) {
	const sources = 20
	const perSource = 10

	f := NewFanInKey(nil)
	for i := 0; i < sources; i++ {
		ids := make([]string, 0, perSource)
		for j := 0; j < perSource; j++ {
			ids = append(ids, fmt.Sprintf("AKIA%d-%d", i, j))
		}
		f.Add(keySource(ids...))
	}
	f.DoneAdding()

	if got := drainKeys(t, f); len(got) != sources*perSource {
		t.Errorf("got %d keys, want %d", len(got), sources*perSource)
	}
}
