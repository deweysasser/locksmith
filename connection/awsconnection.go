package connection

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"sync"
	"time"
)

type AWSConnection struct {
	Type, Profile string
}

func (a *AWSConnection) String() string {
	return fmt.Sprintf("aws://%s", a.Profile)
}

type userMap map[string]*iam.User

func (a *AWSConnection) Fetch() (keys <-chan data.Key, accounts <-chan data.Account) {
	output.Debug("Fetching from aws", a.Profile)
	cKeys := make(chan data.Key)
	cAccounts := make(chan data.Account)

	sharedCredentials := credentials.NewSharedCredentials("", a.Profile)
	region := aws.String("us-east-1")

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		if sess, err := session.NewSession(&aws.Config{
			Region:      region,
			Credentials: sharedCredentials,
		}); err == nil {

			e := ec2.New(sess)

			// TODO:  make this code still fetch keys and instances even if fetching the account ARN fails?
			if arn, err := a.fetchAccountInfo(sess, cAccounts); err == nil {

				wg.Add(1)
				go func() {
					defer wg.Done()
					usermap := a.fetchAccounts(sess, cAccounts, cKeys)
					a.fetchAccessKeys(sess, cAccounts, cKeys, usermap)
				}()
				wg.Add(1)
				go func() {
					defer wg.Done()
					a.fetchAccountInfo(sess, cAccounts)
				}()
				if dro, err := e.DescribeRegions(&ec2.DescribeRegionsInput{}); err == nil {
					for _, r := range dro.Regions {
						wg.Add(1)
						go func(region *string) {
							defer wg.Done()
							keymap := a.fetchKeyPairs(arn, region, sharedCredentials, cKeys, cAccounts)
							a.fetchInstances(region, sharedCredentials, cAccounts, keymap)
						}(r.RegionName)
					}
				} else {
					output.Error(a, "failed to lookup EC2 regions")
				}
			} else {
				output.Error("Failed to get account identity for", a.Profile, ":", err)
			}
		} else {
			output.Error("Failed to create AWS BOTO session:", err)
		}

	}()

	go func() {
		wg.Wait()
		defer close(cKeys)
		defer close(cAccounts)
	}()

	return cKeys, cAccounts
}

func (a *AWSConnection) fetchAccountInfo(sess *session.Session, accounts chan<- data.Account) (data.AWSAccountID, error) {
	s := sts.New(sess)
	i := iam.New(sess)

	if out, err := s.GetCallerIdentity(&sts.GetCallerIdentityInput{}); err == nil {
		arn := data.AWSAccountID(*out.Account)
		if iout, err := i.ListAccountAliases(&iam.ListAccountAliasesInput{}); err == nil {
			aliases := make([]string, len(iout.AccountAliases))
			for _, a := range iout.AccountAliases {
				aliases = append(aliases, *a)
			}
			accounts <- data.NewAWSAccount(arn, a.Id(), []data.KeyBinding{}, aliases...)
		} else {
			output.Warn("Failed to get accont alias for", a.Profile, "account", arn)
			accounts <- data.NewAWSAccount(arn, a.Id(), []data.KeyBinding{})
		}
		return arn, nil
	} else {
		return data.AWSAccountID(""), err
	}
}

func (a *AWSConnection) fetchAccessKeys(sess *session.Session, accounts chan<- data.Account, keys chan<- data.Key, usermap userMap) {

	i := iam.New(sess)

	if lako, err := i.ListAccessKeys(&iam.ListAccessKeysInput{}); err == nil {
		for _, md := range lako.AccessKeyMetadata {
			output.Debug("Found acces key", *md.AccessKeyId)
			userName := *md.UserName
			keys <- data.NewAwsKey(*md.AccessKeyId, *md.CreateDate, *md.Status == "Active", userName, *usermap[userName].Arn)
			//accounts <- data.NewIAMAccount(usermap[*md.UserName], a.Id(), md)
			accounts <- data.NewIAMAccountFromKey(md, usermap[userName], a.Id())
		}
	} else {
		output.Error(a, "failed to list IAM users")
	}
}

func (a *AWSConnection) fetchAccounts(sess *session.Session, accounts chan<- data.Account, keys chan<- data.Key) userMap {
	usermap := make(userMap)
	i := iam.New(sess)

	if r, err := i.ListUsers(&iam.ListUsersInput{}); err == nil {
		for _, user := range r.Users {
			usermap[*user.UserName] = user
		}
	} else {
		output.Error(a, "failed to list IAM users")
	}

	return usermap
}

func (a *AWSConnection) fetchInstances(region *string, sharedCredentials *credentials.Credentials, cAccounts chan data.Account, keymap map[string]data.ID) {
	output.Debug(a, "fetching instances from", *region)
	if sess, err := session.NewSession(&aws.Config{
		Region:      region,
		Credentials: sharedCredentials,
	}); err == nil {
		e := ec2.New(sess)
		//ec2.DescribeInstancesOutput{}.Reservations[0].Instances[0].k
		if dio, err := e.DescribeInstances(&ec2.DescribeInstancesInput{}); err == nil {
			output.Debug(a, *region, "reservations:", len(dio.Reservations))
			for _, res := range dio.Reservations {
				output.Debug(a, *region, "instances:", len(res.Instances))
				for _, instance := range res.Instances {
					keyID := keymap[*instance.KeyName]
					keys := []data.KeyBinding{
						{
							KeyID:    keyID,
							Location: data.INSTANCE_ROOT_CREDENTIALS,
						},
					}

					acct := data.NewAWSInstanceAccount(instance, a.Id(), keys)
					output.Debug("Found instance account", acct)
					cAccounts <- acct
				}
			}
		} else {
			output.Error(a, "Failed to fetch instances")
		}

	} else {
		output.Error(a, "Failed to connect")
	}
}

func (a *AWSConnection) fetchKeyPairs(arn data.AWSAccountID, region *string, sharedCredentials *credentials.Credentials, cKeys chan data.Key, cAccounts chan data.Account) (keymap map[string]data.ID) {
	output.Debug(a, "fetching key pairs from", *region)
	keymap = make(map[string]data.ID)

	if sess, err := session.NewSession(&aws.Config{
		Region:      region,
		Credentials: sharedCredentials,
	}); err == nil {
		e := ec2.New(sess)

		if out, err := e.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{}); err == nil {
			bindings := make([]data.KeyBinding, 0)
			for _, p := range out.KeyPairs {
				fp := p.KeyFingerprint
				name := p.KeyName
				key := data.NewSSHKeyFromFingerprint(*name, time.Now(), data.ID(*fp))
				cKeys <- key
				bindings = append(bindings, data.KeyBinding{KeyID: data.ID(*fp), Name: *name})
				keymap[*name] = data.ID(*fp)
			}

			acct := data.NewAWSAccount(arn, a.Id(), bindings)

			cAccounts <- acct
		} else {
			output.Warn(a.String()+":", "Failed to find key pairs:", err)
		}

	} else {
		output.Error("Failed to connect to", a)
	}

	return
}

func (a *AWSConnection) Id() data.ID {
	return data.ID(a.Profile)
}
