package connection

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
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

		sess, err := session.NewSession(&aws.Config{
			Region:      region,
			Credentials: sharedCredentials,
		})
		if err != nil {
			output.Error("Failed to create AWS BOTO session:", err)
			return
		}

		e := ec2.New(sess)
		iamClient := iam.New(sess)
		stsClient := sts.New(sess)

		// TODO:  make this code still fetch keys and instances even if fetching the account ARN fails?
		arn, err := a.fetchAccountInfo(stsClient, iamClient, cAccounts)
		if err != nil {
			output.Error("Failed to get account identity for", a.Profile, ":", err)
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			usermap := a.fetchAccounts(iamClient, cAccounts, cKeys)
			a.fetchAccessKeys(iamClient, cAccounts, cKeys, usermap)
		}()

		dro, err := e.DescribeRegions(&ec2.DescribeRegionsInput{})
		if err != nil {
			output.Error(a, "failed to lookup EC2 regions")
			return
		}

		for _, r := range dro.Regions {
			wg.Add(1)
			go func(regionName *string) {
				defer wg.Done()

				regionSession, err := session.NewSession(&aws.Config{
					Region:      regionName,
					Credentials: sharedCredentials,
				})
				if err != nil {
					output.Error("Failed to connect to", a)
					return
				}

				regionEC2 := ec2.New(regionSession)

				keymap := a.fetchKeyPairs(regionEC2, arn, aws.StringValue(regionName), cKeys, cAccounts)
				a.fetchInstances(regionEC2, aws.StringValue(regionName), cAccounts, keymap)
			}(r.RegionName)
		}
	}()

	go func() {
		wg.Wait()
		defer close(cKeys)
		defer close(cAccounts)
	}()

	return cKeys, cAccounts
}

func (a *AWSConnection) fetchAccountInfo(s stsiface.STSAPI, i iamiface.IAMAPI, accounts chan<- data.Account) (data.AWSAccountID, error) {
	if out, err := s.GetCallerIdentity(&sts.GetCallerIdentityInput{}); err == nil {
		arn := data.AWSAccountID(aws.StringValue(out.Account))
		if iout, err := i.ListAccountAliases(&iam.ListAccountAliasesInput{}); err == nil {
			aliases := make([]string, 0, len(iout.AccountAliases))
			for _, a := range iout.AccountAliases {
				aliases = append(aliases, aws.StringValue(a))
			}
			accounts <- data.NewAWSAccount(arn, a.Id(), []data.KeyBindingImpl{}, aliases...)
		} else {
			output.Warn("Failed to get accont alias for", a.Profile, "account", arn)
			accounts <- data.NewAWSAccount(arn, a.Id(), []data.KeyBindingImpl{})
		}
		return arn, nil
	} else {
		return data.AWSAccountID(""), err
	}
}

// fetchAccessKeys lists the access keys of every user in usermap and pushes a
// key and an IAM account for each one.
//
// Note that iam.ListAccessKeys only ever reports the keys of a single user:
// the one named in UserName, or -- when UserName is empty -- the user whose
// credentials signed the request.  It must therefore be called once per user.
// The no-UserName form is kept only as a fallback for the case where the
// account has no IAM users at all (the root user then gets its own keys back).
func (a *AWSConnection) fetchAccessKeys(i iamiface.IAMAPI, accounts chan<- data.Account, keys chan<- data.Key, usermap userMap) {
	if len(usermap) == 0 {
		a.fetchAccessKeysForUser(i, "", accounts, keys, usermap)
		return
	}

	for userName := range usermap {
		a.fetchAccessKeysForUser(i, userName, accounts, keys, usermap)
	}
}

func (a *AWSConnection) fetchAccessKeysForUser(i iamiface.IAMAPI, userName string, accounts chan<- data.Account, keys chan<- data.Key, usermap userMap) {
	input := &iam.ListAccessKeysInput{}
	if userName != "" {
		input.UserName = aws.String(userName)
	}

	err := i.ListAccessKeysPages(input, func(lako *iam.ListAccessKeysOutput, _ bool) bool {
		for _, md := range lako.AccessKeyMetadata {
			output.Debug("Found acces key", aws.StringValue(md.AccessKeyId))
			keyUser := aws.StringValue(md.UserName)

			user, ok := usermap[keyUser]
			if !ok {
				// We know about a key but not about the user which owns it -- most
				// likely because ListUsers failed or did not include this user.
				output.Warn(a.String()+":", "no IAM user found for access key", aws.StringValue(md.AccessKeyId), "(user", keyUser+")")
				keys <- data.NewAwsKey(aws.StringValue(md.AccessKeyId),
					aws.TimeValue(md.CreateDate),
					aws.StringValue(md.Status) == "Active",
					keyUser)
				continue
			}

			keys <- data.NewAwsKey(aws.StringValue(md.AccessKeyId),
				aws.TimeValue(md.CreateDate),
				aws.StringValue(md.Status) == "Active",
				keyUser,
				aws.StringValue(user.Arn))
			accounts <- data.NewIAMAccountFromKey(md, user, a.Id())
		}
		return true
	})

	if err != nil {
		output.Error(a, "failed to list access keys for", userName, ":", err)
	}
}

func (a *AWSConnection) fetchAccounts(i iamiface.IAMAPI, accounts chan<- data.Account, keys chan<- data.Key) userMap {
	usermap := make(userMap)

	err := i.ListUsersPages(&iam.ListUsersInput{}, func(r *iam.ListUsersOutput, _ bool) bool {
		for _, user := range r.Users {
			usermap[aws.StringValue(user.UserName)] = user
		}
		return true
	})

	if err != nil {
		output.Error(a, "failed to list IAM users")
	}

	return usermap
}

func (a *AWSConnection) fetchInstances(e ec2iface.EC2API, region string, cAccounts chan<- data.Account, keymap map[string]data.ID) {
	output.Debug(a, "fetching instances from", region)

	err := e.DescribeInstancesPages(&ec2.DescribeInstancesInput{}, func(dio *ec2.DescribeInstancesOutput, _ bool) bool {
		output.Debug(a, region, "reservations:", len(dio.Reservations))
		for _, res := range dio.Reservations {
			output.Debug(a, region, "instances:", len(res.Instances))
			for _, instance := range res.Instances {
				keyID := keymap[aws.StringValue(instance.KeyName)]
				keys := []data.KeyBindingImpl{
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
		return true
	})

	if err != nil {
		output.Error(a, "Failed to fetch instances")
	}
}

func (a *AWSConnection) fetchKeyPairs(e ec2iface.EC2API, arn data.AWSAccountID, region string, cKeys chan<- data.Key, cAccounts chan<- data.Account) (keymap map[string]data.ID) {
	output.Debug(a, "fetching key pairs from", region)
	keymap = make(map[string]data.ID)

	if out, err := e.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{}); err == nil {
		bindings := make([]data.KeyBindingImpl, 0)
		for _, p := range out.KeyPairs {
			fp := aws.StringValue(p.KeyFingerprint)
			name := aws.StringValue(p.KeyName)
			key := data.NewSSHKeyFromFingerprint(name, time.Now(), data.ID(fp))
			cKeys <- key
			bindings = append(bindings, data.KeyBindingImpl{KeyID: data.ID(fp), Name: name})
			keymap[name] = data.ID(fp)
		}

		acct := data.NewAWSAccount(arn, a.Id(), bindings)

		cAccounts <- acct
	} else {
		output.Warn(a.String()+":", "Failed to find key pairs:", err)
	}

	return
}

func (a *AWSConnection) Id() data.ID {
	return data.ID(a.Profile)
}
