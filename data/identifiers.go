package data

import (
	"crypto/sha256"
	"fmt"
)

type ID string

type Ider interface {
	Id() ID
}

type Identiferser interface {
	Identifers() []ID
}

func IdFromString(s string) ID {
	return IdFromBytes([]byte(s))
}

func IdFromBytes(s []byte) ID {
	return ID(fmt.Sprintf("%x", sha256.Sum256(s)))
}
