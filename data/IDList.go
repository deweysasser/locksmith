package data

import (
	"encoding/json"
)

type IDList struct {
	Ids []ID
}

func (l *IDList) Add(i ID) bool {
	if !l.Contains(i) {
		l.Ids = append(l.Ids, i)
		return true
	}
	return false
}

func (l *IDList) AddList(other *IDList) bool {
	retval := false
	for _, id := range other.Ids {
		retval = retval || l.Add(id)
	}

	return retval
}

func (l *IDList) AddArray(other []ID) bool {
	retval := false
	for _, id := range other {
		retval = retval || l.Add(id)
	}

	return retval
}

func (l *IDList) Contains(i ID) bool {
	for _, id := range l.Ids {
		if id == i {
			return true
		}
	}
	return false
}

func (l *IDList) Length() int {
	if l.Ids == nil {
		return 0
	}
	return len(l.Ids)
}

func (l IDList) MarshalJSON() ([]byte, error) {
	if l.Ids == nil {
		return []byte("[]"), nil
	}

	return json.Marshal(l.Ids)
}

func (l *IDList) UnmarshalJSON(data []byte) error {
	//fmt.Println("Bytes are", string(data))
	if len(data) > 0 {
		s := make([]string, 0)

		json.Unmarshal(data, &s)

		for _, str := range s {
			l.Add(ID(str))
		}
	}

	return nil
}
