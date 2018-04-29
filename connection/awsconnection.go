package connection

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/output"
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

	go func() {
		defer close(cKeys)
		defer close(cAccounts)

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String("us-east-1"),
			Credentials: credentials.NewSharedCredentials("", a.Profile),
		})

		if err != nil {
			output.Error("Failed to connect")
			return
		}

		e := ec2.New(sess)

		out, err := e.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{})

		if err != nil {
			output.Warn(a.String()+":", "Failed to find key pairs:", err)
			return
		}

		bindings := make([]data.KeyBinding, 0)
		for _, p := range out.KeyPairs {
			fp := p.KeyFingerprint
			name := p.KeyName
			bindings = append(bindings, data.KeyBinding{KeyID: data.ID(*fp), Name: *name})
		}

		acct := data.NewAWSAccount(a.Profile, a.Id(), bindings)

		cAccounts <- acct

	}()

	return cKeys, cAccounts
}

func (a *AWSConnection) Id() data.ID {
	return data.ID(a.Profile)
}
