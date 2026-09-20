package data

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"sort"
	"strings"
	"time"
)

type AWSAccountID string
type ARN string

type accountImpl struct {
	Type       string
	Connection ID
	Keys       []KeyBindingImpl

	// Observed names the binding locations this object is a *complete*
	// observation of.  A fetch that read a host's authorized_keys file sets
	// AUTHORIZED_KEYS here even when it found no keys at all, which is what
	// lets Merge tell "I looked and there was nothing" apart from "I did not
	// look".  Without it every merge is a union, bindings only ever
	// accumulate, and a key removed from a host is reported as still present
	// forever.
	//
	// Not persisted: it describes the observation, not the stored record.
	Observed []BindingLocation `json:"-"`
}

// MarkObserved records that this object is a complete observation of the given
// binding locations.  A connection must only claim a location it genuinely
// enumerated in full: claiming one it merely sampled silently deletes real
// bindings on the next merge.
func (a *accountImpl) MarkObserved(locations ...BindingLocation) {
	a.Observed = append(a.Observed, locations...)
}

type SSHAccount struct {
	accountImpl
	Username, Host string
}

type AWSAccount struct {
	accountImpl
	Arn     AWSAccountID
	Aliases StringSet
}

type AWSIamAccount struct {
	accountImpl
	Arn        ARN
	Username   string
	CreateDate time.Time
}

type AWSInstanceAccount struct {
	accountImpl
	InstanceId, NameTag, PublicDNS string
}

type Account interface {
	Ider
	Bindings() <-chan KeyBindingImpl
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
			Type:       "AWSIamAccount",
			Connection: conn,
			Keys:       []KeyBindingImpl{},
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
			Type:       "AWSInstanceAccount",
			Connection: connID,
			Keys:       keys},
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
		host = name[(i + 1):]
	}

	return &SSHAccount{accountImpl{Type: "SSHAccount", Connection: connID, Keys: keys}, username, host}
}

func NewAWSAccount(arn AWSAccountID, connID ID, keys []KeyBindingImpl, aliases ...string) *AWSAccount {
	sAliases := StringSet{}
	sAliases.AddArray(aliases)
	return &AWSAccount{accountImpl{Type: "AWSAccount", Connection: connID, Keys: keys}, arn, sAliases}
}

func (a *accountImpl) Merge(account accountImpl) {
	// The incoming object is the fresh observation, so its Observed list is
	// the one that carries authority.
	a.Keys = mergeBindings(a.Keys, account.Keys, account.Observed)
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

func (a *accountImpl) Bindings() <-chan KeyBindingImpl {
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

// AddBinding records that a key is bound here, with whatever authorized_keys
// options accompanied it.  options is "" for sources that have no notion of
// them (AWS, Digital Ocean) or for an unrestricted line.
func (a *accountImpl) AddBinding(k Key, location BindingLocation, options string) {
	a.Keys = append(a.Keys, KeyBindingImpl{KeyID: k.Id(), Location: location, Options: options})
}

//func (a *accountImpl) Id() ID {
//	return ID(a.Type + "_" + a.Name)
//}

// mergeBindings combines the bindings already recorded for an account with
// those just observed, returning a unique, stably ordered set.
//
// authoritative names the locations the observation enumerated in full.  A
// recorded binding at such a location that the observation did not see is
// dropped: it is no longer on the system.  Locations absent from the list are
// left alone, because one account legitimately carries bindings from several
// sources -- authorized_keys from an SSH fetch, instance credentials from AWS --
// and a fetch that surveyed one must not discard the others.
//
// Note this drops *bindings*, never keys.  The key record is a permanent
// catalog entry; a binding is a claim about one system at one point in time.
func mergeBindings(existing []KeyBindingImpl, observed []KeyBindingImpl, authoritative []BindingLocation) []KeyBindingImpl {
	claimed := make(map[BindingLocation]bool, len(authoritative))
	for _, l := range authoritative {
		claimed[l] = true
	}

	s := StringSet{}
	for _, k := range existing {
		if claimed[k.Location] {
			continue
		}
		s.Add(toJson(&k))
	}

	for _, k := range observed {
		s.Add(toJson(&k))
	}

	// Sort the encoded bindings so that merging the same set twice yields the
	// same order.  Accounts are re-merged and rewritten on every fetch, and the
	// repository is meant to be kept in git, so an unstable order would churn
	// the stored JSON for no reason.
	encoded := s.StringArray()
	sort.Strings(encoded)

	result := make([]KeyBindingImpl, 0, len(encoded))

	for _, e := range encoded {
		result = append(result, fromJson(e))
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
