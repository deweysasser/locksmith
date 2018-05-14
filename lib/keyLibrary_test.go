package lib

import (
	"testing"

	"github.com/deweysasser/locksmith/data"
	"os"
	"time"
)

func Test_keyLibrary_Basic(t *testing.T) {
	lib := NewKeyLibrary("test-output")

	k := data.NewAwsKey("testing", time.Time{}, true)

	lib.Store(k)

	_, err := os.Stat("test-output/testing.json")
	check(t, err, "Stat key storage")

	if k2, err := lib.Fetch(data.ID("testing")); err != nil {
		if key2, ok := k2.(*data.AWSKey); ok {
			assertStringEquals(t, "Key restored", k.AwsKeyId, key2.AwsKeyId)
		}
	}
}
