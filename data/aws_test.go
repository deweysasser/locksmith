package data

import (
	"encoding/json"
	"testing"
)

func TestBasic(t *testing.T) {
	impl := NewAwsKey("12345", "")

	var key Key
	key = impl

	assertStringsEquals(t, "12345", string(key.Id()))

	if s, e := impl.Json(); e == nil {
		assertStringsEquals(t, `{
  "Type": "AWSKey",
  "Names": [],
  "AwsKeyId": "12345",
  "AwsSecretKey": "",
  "Active": true
}`, string(s))
	} else {
		t.Error("json failed", e)
	}
}

func testJson(i interface{}) ([]byte, error) {
	return json.MarshalIndent(i, "", "  ")
}
