package connection

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
	"sync"
)

type AWSConnection struct {
	Type, Profile string
}

func (a *AWSConnection) String() string {
	return fmt.Sprintf("aws://%s", a.Profile)
}

func (a *AWSConnection) Fetch() (keys <- chan data.Key, accounts <- chan data.Account) {
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

			if dro, err := e.DescribeRegions(&ec2.DescribeRegionsInput{}); err == nil {
				for _, r := range dro.Regions {
					wg.Add(1)
					go func() {
						defer wg.Done()
						a.fetchKeyPairs(r.RegionName, sharedCredentials, cAccounts)
					}()
				}
			} else {
				output.Error(a, "failed to lookup EC2 regions")
			}

		}

	}()

	go func() {
		wg.Wait()
		defer close(cKeys)
		defer close(cAccounts)
	}()

	return cKeys, cAccounts
}

func (a *AWSConnection) fetchKeyPairs(region *string, sharedCredentials *credentials.Credentials, cAccounts chan data.Account) {
	output.Debug(a, "fetching key pairs from", *region)
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
				bindings = append(bindings, data.KeyBinding{KeyID: data.ID(*fp), Name: *name})
			}

			acct := data.NewAWSAccount(a.Profile, a.Id(), bindings)

			cAccounts <- acct
		} else {
			output.Warn(a.String()+":", "Failed to find key pairs:", err)
		}

	} else {
		output.Error("Failed to connect to", a)
	}
}

func (a *AWSConnection) Id() data.ID {
	return data.ID(a.Profile)
}
