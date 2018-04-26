package data

import (
	"encoding/json"
	"strings"
)

type StringSet struct {
	strings map[string]bool
}

func (set *StringSet) Add(s string) {
	if set.strings == nil {
		set.strings = make(map[string]bool)
	}

	set.strings[s]=true
}

func (set *StringSet) AddSet(other StringSet) {
	if set.strings == nil {
		set.strings = make(map[string]bool)
	}

	if other.strings != nil{
		for v, _ := range other.strings {
			if v != "" {
				set.strings[v] = true
			}
		}
	}
}

func (s *StringSet) Count() int {
	if nil == s.strings {
		return 0
	}
	return len(s.strings)
}

func (s *StringSet) Join(sep string) string {
	if nil == s.strings {
		return ""
	}
	return strings.Join(s.StringArray(), sep)
}

func (s *StringSet) Contains(str string) bool {
	if nil == s.strings {
		return false
	}
	_, ok := s.strings[str]
	return ok
}

func (s *StringSet) Values() chan string {
	c := make(chan string)
	if nil == s.strings {
		close(c)
	} else {
		go func() {
			defer close(c)
			for k, _ := range (s.strings) {
				c <- k
			}
		}()
	}

	return c
}

func (s *StringSet) MarshalJSON() ([]byte, error) {
	s2 := s.StringArray()

	return json.Marshal(s2)
}

func (s *StringSet) StringArray() []string {
	s2 := make([]string, 0)
	for v := range (s.Values()) {
		s2 = append(s2, v)
	}
	return s2
}

func (s *StringSet) UnmarshalJSON(data []byte) error {
	s2 := make([]string, 0)
	if e := json.Unmarshal(data, &s2); e != nil {
		return e
	}
	if s.strings == nil {
		s.strings = make(map[string]bool)
	}
	for _, v := range(s2) {
		s.strings[v]=true
	}

	return nil
}
