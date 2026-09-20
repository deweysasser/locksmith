package data

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	impl := NewAwsKey("12345", time.Time{}, true, "")

	var key Key
	key = impl

	assertStringsEquals(t, "12345", string(key.Id()))

	if s, e := impl.Json(); e == nil {
		assertStringsEquals(t, `{
  "Type": "AWSKey",
  "Names": [],
  "Earliest": "0001-01-01T00:00:00Z",
  "AwsKeyId": "12345",
  "Active": true
}`, string(s))
	} else {
		t.Error("json failed", e)
	}
}

func testJson(i interface{}) ([]byte, error) {
	return json.MarshalIndent(i, "", "  ")
}

// The repository is meant to be shareable, so secret key material must never
// reach the serialized form -- not even as an empty field that a later change
// could start populating.
func TestAWSKeySecretIsNeverSerialized(t *testing.T) {
	key := NewAwsKey("AKIAEXAMPLE", time.Time{}, true, "prod")
	key.AwsSecretKey = "super-secret-should-not-appear"

	bytes, err := key.Json()
	if err != nil {
		t.Fatalf("Json: %v", err)
	}

	if strings.Contains(string(bytes), "super-secret-should-not-appear") {
		t.Errorf("secret key material was serialized:\n%s", bytes)
	}
	if strings.Contains(string(bytes), "AwsSecretKey") {
		t.Errorf("the AwsSecretKey field should not appear at all:\n%s", bytes)
	}
}
