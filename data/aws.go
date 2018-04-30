package data

import (
	"encoding/json"
	"fmt"
	"github.com/deweysasser/locksmith/output"
	"regexp"
	"strings"
	"time"
)

type AWSKey struct {
	keyImpl
	AwsKeyId, AwsSecretKey string
	Active                 bool
	CreateDate time.Time
}

func NewAwsKey(id, name string, createDate time.Time) *AWSKey {
	names := StringSet{}
	if name != "" {
		names.Add(name)
	}
	return &AWSKey{
		keyImpl: keyImpl{
			Type:        "AWSKey",
			Names:       names,
			Deprecated:  false,
			Replacement: "",
	},
		AwsKeyId:     id,
		AwsSecretKey: "",
		Active:       true,
	CreateDate: createDate,
	}
}

// Does nothing for AWS keys
func (key *AWSKey) Merge(k Key) {
	if ak, ok := k.(*AWSKey); ok {
		key.keyImpl.Merge(&ak.keyImpl)
		if key.CreateDate.IsZero() {
			key.CreateDate = ak.CreateDate
		}
	}
}

func (key *AWSKey) Id() ID {
	return ID(key.AwsKeyId)
}

func (key *AWSKey) Identifiers() []ID {
	return []ID{key.Id()}
}

func (key *AWSKey) String() string {
	return fmt.Sprintf("AWS %s (%s)", key.Id(), key.Names.Join(", "))
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
		key := NewAwsKey(fields["aws_access_key_id"], name, time.Time{})
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
