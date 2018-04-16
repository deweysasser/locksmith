package data

import (
	"encoding/json"
)

type AWSKey struct {
	keyImpl
	AwsKeyId, AwsSecretKey string
	Active bool
}

func NewAwsKey(id string, secret string) *AWSKey {
	return &AWSKey{
		keyImpl: keyImpl{
			Type: "AWSKey",
			Ids: []KeyID{KeyID(id)},
			Names: []string{},
			Deprecated: false,
			Replacement: ""},
		AwsKeyId: id,
		AwsSecretKey: "",
		Active: true}
}

func (key *AWSKey) GetNames() []string {
	return key.Names
}

func (key *AWSKey) Ids() []string {
	return []string{key.AwsKeyId}
}

func (key *AWSKey) IsDeprecated() bool {
	return key.Deprecated
}

func (key *AWSKey) Json() ([]byte, error) {
	return json.MarshalIndent(key, "", "  ")
}

func (key *AWSKey) Replacement() KeyID {
	return ""
}
