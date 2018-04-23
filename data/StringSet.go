package data

import (
	"encoding/json"
	"strings"
)

type StringSet map[string]bool

func (set StringSet) Add(s string) {
	set[s]=true
}

func (set StringSet) AddSet(other StringSet) {
	if other != nil {
		for v, _ := range other {
			set[v] = true
		}
	}
}

func (s StringSet) Count() int {
	return len(s)
}

func (s StringSet) Join(sep string) string {
	return strings.Join(s.StringArray(), sep)
}

func (s StringSet) Contains(str string) bool {
	_, ok := s[str]
	return ok
}

func (s StringSet) Values() chan string {
	c := make(chan string)

	go func() {
		defer close(c)
		for k, _ := range(s) {
			c <- k
		}
	}()

	return c
}

func (s StringSet) MarshalJSON() ([]byte, error) {
	s2 := s.StringArray()

	return json.Marshal(s2)
}

func (s StringSet) StringArray() []string {
	s2 := make([]string, 0)
	for v := range (s.Values()) {
		s2 = append(s2, v)
	}
	return s2
}

func (s StringSet) UnmarshalJSON(data []byte) error {

	s2 := make([]string, 0)
	if e := json.Unmarshal(data, &s2); e != nil {
		return e
	}

	s = make(StringSet)
	for _, v := range(s2) {
		s[v]=true
	}

	return nil
}
