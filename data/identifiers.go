package data

import (
	"fmt"
	"crypto/sha256"
)

type ID string

func IdFromString(s string) ID {
	return IdFromBytes([]byte(s))
}

func IdFromBytes(s []byte) ID {
	return ID(fmt.Sprintf("%x", sha256.Sum256(s)))
}
