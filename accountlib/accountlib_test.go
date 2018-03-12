package accountlib

import (
	"github.com/deweysasser/locksmith/keys"
	"os"
	"testing"
)


func assertIntsEqual(t *testing.T, s1, s2 int) {
	if s1 != s2 {
		t.Logf("Expected [%d] but got [%d]", s1, s2)
		t.Fail()
	}
}



func failError(t *testing.T, msg string, e error) {
	if e != nil {
		t.Errorf(msg)
	}
}

func TestEmptyAccountlib(t *testing.T) {
	lib := New("test-output-missing")
	accounts, err  := lib.GetAccounts()

	failError(t, "Failed to read accounts", err)

	assertIntsEqual(t, 0, len(accounts))
}

func TestAccountlibBasic(t *testing.T) {
	lib := New("test-output")

	acc := lib.EnsureAccount("testing.example.com")

	key := keys.Read("../keys/test-data/rsa.pub")

	acc.SetKeys([]keys.Key{key})

	_, err := os.Stat("test-output/accounts/SSH/testing.example.com.json")
	if err != nil {
		t.Error("Falied to save file")
	}

	lib2 := New("test-output")

	accounts, err := lib2.GetAccounts()

	failError(t, "Fetching Accounts", err)

	assertIntsEqual(t, 1, len(accounts))
}
