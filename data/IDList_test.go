package data

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestIDList_Add(t *testing.T) {
	l := IDList{}

	l.Add(ID("foo"))

	if !l.Contains(ID("foo")) {
		t.Error("Failed to add foo")
	}

	if bytes, e := json.Marshal(l); e == nil {
		assertStringsEquals(t, `["foo"]`, string(bytes))
	} else {
		t.Error(e)
	}
}

func TestIDList_AddNoDuplicates(t *testing.T) {
	l := IDList{}

	if l.Length() != 0 {
		t.Error("Wrong length of zero length list")
	}

	l.Add(ID("foo"))
	l.Add(ID("bar"))
	l.Add(ID("foo"))
	l.Add(ID("bar"))
	l.Add(ID("foo"))
	l.Add(ID("bar"))

	if !l.Contains(ID("foo")) {
		t.Error("Failed to add foo")
	}
	if !l.Contains(ID("bar")) {
		t.Error("Failed to add bar")
	}

	if l.Length() > 2 {
		t.Error("Too many entries")
	}
}

func TestMarshal(t *testing.T) {
	l := IDList{}

	l.Add(ID("foo"))
	l.Add(ID("bar"))

	if bytes, err := json.Marshal(&l); err == nil {
		assertStringsEquals(t, `["foo","bar"]`, string(bytes))
	} else {
		t.Error(err)
	}
}

func TestIDList_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		fields  IDList
		want    string
		wantErr bool
	}{
		{"empty",
			IDList{},
			"[]",
			false,
		},
		{"empty2",
			*new(IDList),
			"[]",
			false,
		},
		{"single field",
			IDList{[]ID{ID("foo")}},
			`["foo"]`,
			false,
		},
		{"multiple fields",
			IDList{[]ID{ID("foo"), ID("bar")}},
			`["foo","bar"]`,
			false,
		},
		{"duplicates",
			IDList{[]ID{ID("foo"), ID("foo")}},
			`["foo","foo"]`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.fields)
			if (err != nil) != tt.wantErr {
				t.Errorf("IDList.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			sGot := string(got)
			if sGot != tt.want {
				t.Errorf("IDList.MarshalJSON() = '%v', want '%v'", sGot, tt.want)
			}
		})
	}
}

func TestIDList_UnmarshalJSONs(t *testing.T) {
	tests := []struct {
		name    string
		want    IDList
		args    string
		wantErr bool
	}{
		{"empty",
			IDList{},
			"[]",
			false,
		},
		{"single field",
			IDList{[]ID{ID("foo")}},
			`["foo"]`,
			false,
		},
		{"multiple fields",
			IDList{[]ID{ID("foo"), ID("bar")}},
			`["foo", "bar"]`,
			false,
		},
		{"duplicates",
			IDList{[]ID{ID("foo")}},
			`["foo","foo"]`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &IDList{}
			if err := json.Unmarshal([]byte(tt.args), l); (err != nil) != tt.wantErr {
				t.Errorf("IDList.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				if !reflect.DeepEqual(l, &tt.want) {
					t.Errorf("IDList.UnmarshalJSON() result = %v, want = %v", *l, tt.want)
				}
			}
		})
	}
}

func TestIDList_UnmarshalJSONs2(t *testing.T) {
	tests := []struct {
		name    string
		want    IDList
		args    string
		wantErr bool
	}{
		{"empty",
			IDList{},
			`{"Ids":[]}`,
			false,
		},
		{"single field",
			IDList{[]ID{ID("foo")}},
			`{"Ids":["foo"]}`,
			false,
		},
		{"multiple fields",
			IDList{[]ID{ID("foo"), ID("bar")}},
			`{"Ids":["foo", "bar"]}`,
			false,
		},
		{"duplicates",
			IDList{[]ID{ID("foo")}},
			`{"Ids":["foo","foo"]}`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := struct {
				Ids IDList
			}{}

			if err := json.Unmarshal([]byte(tt.args), &l); (err != nil) != tt.wantErr {
				t.Errorf("IDList.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				if !reflect.DeepEqual(l.Ids, tt.want) {
					t.Errorf("IDList.UnmarshalJSON() result = %v, want = %v", l, tt.want)
				}
			}
		})
	}
}
