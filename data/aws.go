package data

import (
	"encoding/json"
	"github.com/deweysasser/locksmith/output"
	"regexp"
	"strings"
	"time"
)

type AWSKey struct {
	keyImpl
	AwsKeyId, AwsSecretKey string
	Active                 bool
}

func NewAwsKey(id string, createDate time.Time, active bool, aNames ...string) *AWSKey {
	sNames := StringSet{}
	sNames.AddArray(aNames)
	return &AWSKey{
		keyImpl: keyImpl{
			Type:        "AWSKey",
			Names:       sNames,
			Deprecated:  false,
			Replacement: "",
			Earliest:    createDate,
	},
		AwsKeyId:     id,
		AwsSecretKey: "",
		Active:       active,
	}
}

// Does nothing for AWS keys
func (key *AWSKey) Merge(k Key) {
	if ak, ok := k.(*AWSKey); ok {
		key.keyImpl.Merge(&ak.keyImpl)
	}
}

func (key *AWSKey) Id() ID {
	return ID(key.AwsKeyId)
}

func (key *AWSKey) Identifiers() []ID {
	return []ID{key.Id()}
}

func (key *AWSKey) String() string {
	return key.keyImpl.StandardString(key.Id())
}

func (key *AWSKey) Ids() []string {
	return []string{key.AwsKeyId}
}

func (key *AWSKey) Json() ([]byte, error) {
	return json.MarshalIndent(key, "", "  ")
}

func ParseAWSCredentials(bytes []byte, keys chan Key) {

	config := parseFile(string(bytes))

	for name, fields := range config {
		output.Debug("Reading key", name)
		key := NewAwsKey(fields["aws_access_key_id"], time.Time{}, true, name)
		keys <- key
	}
}

func parseFile(input string) map[string]map[string]string {
	reSection := regexp.MustCompile(`^\[(.*)\]`)
	reField := regexp.MustCompile(`^\s*(\S+)\s*=\s*(\S+)\s*$`)

	all := make(map[string]map[string]string)

	var current map[string]string

	for _, line := range strings.Split(input, "\n") {

		parts := reSection.FindAllStringSubmatch(line, -1)
		if parts != nil {
			current = make(map[string]string)
			all[parts[0][1]] = current
		} else {
			fieldParts := reField.FindAllStringSubmatch(line, -1)
			if fieldParts != nil {
				current[fieldParts[0][1]] = fieldParts[0][2]
			}

		}
	}

	return all

}
