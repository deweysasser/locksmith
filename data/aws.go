package data

import (
	"encoding/json"
	"strings"
	"regexp"
	"fmt"
)

type AWSKey struct {
	keyImpl
	AwsKeyId, AwsSecretKey string
	Active                 bool
}

func NewAwsKey(id, name string ) *AWSKey {
	return &AWSKey{
		keyImpl: keyImpl{
			Type:        "AWSKey",
			Names:       StringSet{},
			Deprecated:  false,
			Replacement: ""},
		AwsKeyId:     id,
		AwsSecretKey: "",
		Active:       true}
}

// Does nothing for AWS keys
func (key *AWSKey) Merge(k Key) {
	return
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

func (key *AWSKey) GetNames() []string {
	return key.Names.StringArray()
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

func (key *AWSKey) Replacement() ID {
	return ""
}

func ParseAWSCredentials(bytes []byte, keys chan Key) {

	config := parseFile(string(bytes))

	for name, fields := range(config) {
		key := NewAwsKey(fields["aws_access_key_id"], name)
		keys <- key
		}
}

func parseFile(input string) map[string]map[string]string {
	reSection := regexp.MustCompile(`^\[(.*)\]`)
	reField := regexp.MustCompile(`^\s*(\S+)\s*=\s*(\S+)\s*$`)

	all := make(map[string]map[string]string)

	var current map[string]string


	for _, line := range(strings.Split(input, "\n")) {

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