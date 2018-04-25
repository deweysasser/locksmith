package data

import (
	"testing"
	"encoding/json"
	"strings"
)

func TestStringSet(t *testing.T) {
	var s  = StringSet{}

	s.Add("foo")
	s.Add("foo")
	s.Add("bar")
	s.Add("bar")

	assertTrue(t, "Length", s.Count() == 2)
	assertTrue(t, "contains foo", s.Contains("foo"))
	assertTrue(t, "contains foo", s.Contains("bar"))
}

func TestJSON(t *testing.T) {
	var s  = StringSet{}

	s.Add("foo")
	s.Add("bar")

	bJson, err := json.Marshal(&s)
	checke(t, err)

	assertTrue(t, "serialized has foo",  strings.Contains(string(bJson), "foo"))

	var s2 = StringSet{}
	err = json.Unmarshal(bJson, &s2)

	checke(t, err)

	assertTrue(t, "deser contains foo", s2.Contains("foo"))
	assertTrue(t, "deser contains bar", s2.Contains("bar"))

	 s3 := StringSet{}

	if bJson, err = json.Marshal(&s3); err != nil {
		t.Error(err)
	} else {
		assertStringsEquals(t, `[]`, string(bJson))
	}
}
