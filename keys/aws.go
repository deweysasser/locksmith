package keys

import (
	"encoding/json"
)

type AWSKey struct {
	keyImpl
	AwsKeyId, AwsSecretKey string
	Active bool
}

func NewAwsKey(id string, secret string) *AWSKey {
	return &AWSKey{keyImpl{"AWSKey",[]KeyID{KeyID(id)}, []string{}, false, ""}, id, "", true}
}

func (key *AWSKey) GetNames() []string {
	return key.Names
}

func (key *AWSKey) Id() string {
	return key.AwsKeyId
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
