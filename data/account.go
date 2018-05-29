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
	Connection ID
	Keys       []KeyBindingImpl
}

type SSHAccount struct {
	accountImpl
	Username, Host string
}

type AWSAccount struct {
	accountImpl
	Arn AWSAccountID
	Aliases StringSet
}

type AWSIamAccount struct {
	accountImpl
	Arn        ARN
	Username string
	CreateDate time.Time
}

type AWSInstanceAccount struct {
	accountImpl
	InstanceId, NameTag, PublicDNS string
}

type Account interface {
	Ider
	Bindings() <- chan KeyBindingImpl
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
	}
}

func NewIAMAccount(md *iam.User, conn ID) *AWSIamAccount {
	return &AWSIamAccount{
		accountImpl{
			"AWSIamAccount",
			conn,
			[]KeyBindingImpl{},
		},
		ARN(*md.Arn),
		*md.UserName,
		*md.CreateDate,
	}
}

func NewIAMAccountFromKey(md *iam.AccessKeyMetadata, userMd *iam.User, conn ID) *AWSIamAccount {
	a := NewIAMAccount(userMd, conn)
	a.Keys = []KeyBindingImpl{
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

func NewAWSInstanceAccount(instance *ec2.Instance, connID ID, keys []KeyBindingImpl) *AWSInstanceAccount {
	acct := &AWSInstanceAccount{
		accountImpl{
			"AWSInstanceAccount",
			connID,
			keys},
		*instance.InstanceId,
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
	s := a.InstanceId
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

func NewSSHAccount(username string, name string, connID ID, keys []KeyBindingImpl) *SSHAccount {
	host := name
	if i := strings.Index(name, "@"); i > -1 {
		host =  name[(i+1):]
	}

	return &SSHAccount{accountImpl{"SSHAccount", connID, keys}, username, host}
}

func NewAWSAccount(arn AWSAccountID, connID ID, keys []KeyBindingImpl, aliases ...string) *AWSAccount {
	sAliases := StringSet{}
	sAliases.AddArray(aliases)
	return &AWSAccount{accountImpl{"AWSAccount", connID, keys}, arn,sAliases}
}

func (a *accountImpl) Merge(account accountImpl) {
	a.Keys = mergeBindings(a.Keys, account.Keys)
}

func (a *AWSIamAccount) String() string {
	if a.Arn != "" {
		return string(a.Arn)
	}
	return fmt.Sprintf("iam:%s", a.Arn)
}

func (a *SSHAccount) Merge(account Account) {
	a.accountImpl.Merge(account.(*SSHAccount).accountImpl)
}

func (a *SSHAccount) Id() ID {
	return ID(fmt.Sprintf("%s@%s", a.Username, a.Host))
}

func (a *AWSInstanceAccount) Merge(account Account) {
	other := account.(*AWSInstanceAccount)
	a.accountImpl.Merge(other.accountImpl)
	a.PublicDNS = other.PublicDNS
}

func (a *AWSInstanceAccount) Id() ID {
	return ID(a.InstanceId)
}

func (a *SSHAccount) String() string {
	return fmt.Sprintf("SSH %s@%s", a.Username, a.Host)
}

func (a *AWSAccount) Merge(account Account) {
	a.accountImpl.Merge(account.(*AWSAccount).accountImpl)
}

func (a *AWSAccount) Id() ID {
	return ID(a.Arn)
}

func (a *AWSAccount) String() string {
	if a.Aliases.Count() < 1 {
		return fmt.Sprintf("aws %s", a.Arn)
	} else {
		return fmt.Sprintf("%s (%s)", a.Arn, a.Aliases.Join(", "))
	}
}

func (a *accountImpl) Bindings() <- chan KeyBindingImpl {
	c := make(chan KeyBindingImpl)

	go func() {
		defer close(c)
		for _, k := range a.Keys {
			c <- k
		}
	}()

	return c
}

//func (a *accountImpl) String() string {
//	return fmt.Sprintf("%s", a.Name)
//}

func (a *accountImpl) AddBinding(k Key) {
	a.Keys = append(a.Keys, KeyBindingImpl{KeyID: k.Id() /* AccountID: a.Id() */})
}

//func (a *accountImpl) Id() ID {
//	return ID(a.Type + "_" + a.Name)
//}

// mergeBindings merges 2 arrays of keybindings resulting in an array of unique keyBindings
// The implementation is a bit of a hack right now
func mergeBindings(b1 []KeyBindingImpl, b2 []KeyBindingImpl) []KeyBindingImpl {
	s := StringSet{}
	for _, k := range b1 {
		s.Add(toJson(&k))
	}

	for _, k := range b2 {
		s.Add(toJson(&k))
	}

	var result []KeyBindingImpl

	for s := range s.Values() {
		result = append(result, fromJson(s))
	}

	return result
}

func toJson(binding *KeyBindingImpl) string {
	if bytes, err := json.Marshal(binding); err == nil {
		return string(bytes)
	} else {
		panic(fmt.Sprintf("Failed to jsonify keybinding: %s", err))
	}
}

func fromJson(s string) KeyBindingImpl {
	var k KeyBindingImpl

	if e := json.Unmarshal([]byte(s), &k); e == nil {
		return k
	} else {
		panic(fmt.Sprintf("Failed to unmarshal keybinding: %s", e))
	}
}
