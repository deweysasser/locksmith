package data

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"strings"
	"time"
)

type AWSAccountID string
type ARN string

type accountImpl struct {
	Type       string
	Name       string
	Connection ID
	Keys       []KeyBinding
}

type SSHAccount struct {
	accountImpl
}

type AWSAccount struct {
	accountImpl
	Aliases StringSet
}

type AWSIamAccount struct {
	accountImpl
	Arn        ARN
	CreateDate time.Time
}

type AWSInstanceAccount struct {
	accountImpl
	NameTag, PublicDNS string
}

type Account interface {
	Ider
	Bindings() []KeyBinding
	Merge(a Account)
	ConnectionID() ID
}

func (a *accountImpl) ConnectionID() ID {
	return a.Connection
}

func (a *AWSIamAccount) Id() ID {
	return ID(a.Arn)
}

func (a *AWSIamAccount) Identifiers() []ID {
	return []ID{
		ID(a.Arn),
		a.accountImpl.Id(),
	}
}

func NewIAMAccount(md *iam.User, conn ID) *AWSIamAccount {
	return &AWSIamAccount{
		accountImpl{
			"AWSIamAccount",
			*md.UserName,
			conn,
			[]KeyBinding{},
		},
		ARN(*md.Arn),
		*md.CreateDate,
	}
}

func NewIAMAccountFromKey(md *iam.AccessKeyMetadata, userMd *iam.User, conn ID) *AWSIamAccount {
	a := NewIAMAccount(userMd, conn)
	a.Keys = []KeyBinding{
		{
			KeyID: ID(*md.AccessKeyId),
		},
	}
	return a
}

func (a *AWSIamAccount) Merge(other Account) {
	otherAcc := other.(*AWSIamAccount)
	a.accountImpl.Merge(otherAcc.accountImpl)
	if a.CreateDate.IsZero() {
		a.CreateDate = otherAcc.CreateDate
	}
	if a.Arn == "" {
		a.Arn = otherAcc.Arn
	}
}

func NewAWSInstanceAccount(instance *ec2.Instance, connID ID, keys []KeyBinding) *AWSInstanceAccount {
	acct := &AWSInstanceAccount{
		accountImpl{
			"AWSInstanceAccount",
			*instance.InstanceId,
			connID,
			keys},
		"",
		*instance.PublicDnsName}
	for _, tag := range instance.Tags {
		if "Name" == *tag.Key {
			acct.NameTag = *tag.Value
		}
	}

	return acct
}

func (a *AWSInstanceAccount) String() string {
	s := a.accountImpl.String()
	var parts []string
	if a.NameTag != "" {
		parts = append(parts, a.NameTag)
	}
	if a.PublicDNS != "" {
		parts = append(parts, a.PublicDNS)
	}

	if len(parts) == 0 {
		return s
	} else {
		return fmt.Sprintf("%s (%s)", s, strings.Join(parts, ", "))
	}
}

func NewSSHAccount(name string, connID ID, keys []KeyBinding) *SSHAccount {
	return &SSHAccount{accountImpl{"SSHAccount", name, connID, keys}}
}

func NewAWSAccount(arn AWSAccountID, connID ID, keys []KeyBinding, aliases ...string) *AWSAccount {
	sAliases := StringSet{}
	sAliases.AddArray(aliases)
	return &AWSAccount{accountImpl{"AWSAccount", string(arn), connID, keys}, sAliases}
}

func (a *accountImpl) Merge(account accountImpl) {
	a.Keys = mergeBindings(a.Keys, account.Keys)
}

func (a *AWSIamAccount) String() string {
	if a.Arn != "" {
		return string(a.Arn)
	}
	return fmt.Sprintf("iam:%s", a.Name)
}

func (a *SSHAccount) Merge(account Account) {
	a.accountImpl.Merge(account.(*SSHAccount).accountImpl)
}

func (a *AWSInstanceAccount) Merge(account Account) {
	other := account.(*AWSInstanceAccount)
	a.accountImpl.Merge(other.accountImpl)
	a.PublicDNS = other.PublicDNS
}

func (a *SSHAccount) String() string {
	return fmt.Sprintf("SSH %s", a.Name)
}

func (a *AWSAccount) Merge(account Account) {
	a.accountImpl.Merge(account.(*AWSAccount).accountImpl)
}

func (a *AWSAccount) String() string {
	if a.Aliases.Count() < 1 {
		return fmt.Sprintf("aws %s", a.Name)
	} else {
		return fmt.Sprintf("%s (%s)", a.accountImpl.String(), a.Aliases.Join(", "))
	}
}

func (a *accountImpl) Bindings() []KeyBinding {
	return a.Keys
}

func (a *accountImpl) String() string {
	return fmt.Sprintf("%s", a.Name)
}

func (a *accountImpl) AddBinding(k Key) {
	a.Keys = append(a.Keys, KeyBinding{KeyID: k.Id() /* AccountID: a.Id() */})
}

func (a *accountImpl) Id() ID {
	return ID(a.Type + "_" + a.Name)
}

// mergeBindings merges 2 arrays of keybindings resulting in an array of unique keyBindings
// The implementation is a bit of a hack right now
func mergeBindings(b1 []KeyBinding, b2 []KeyBinding) []KeyBinding {
	s := StringSet{}
	for _, k := range b1 {
		s.Add(toJson(&k))
	}

	for _, k := range b2 {
		s.Add(toJson(&k))
	}

	var result []KeyBinding

	for s := range s.Values() {
		result = append(result, fromJson(s))
	}

	return result
}

func toJson(binding *KeyBinding) string {
	if bytes, err := json.Marshal(binding); err == nil {
		return string(bytes)
	} else {
		panic(fmt.Sprintf("Failed to jsonify keybinding: %s", err))
	}
}

func fromJson(s string) KeyBinding {
	var k KeyBinding

	if e := json.Unmarshal([]byte(s), &k); e == nil {
		return k
	} else {
		panic(fmt.Sprintf("Failed to unmarshal keybinding: %s", e))
	}
}
