package connection

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/deweysasser/locksmith/data"
	"fmt"
	"github.com/deweysasser/locksmith/output"
)

type AWSConnection struct {
	Type, Profile string
}

func (a *AWSConnection) String() string {
	return fmt.Sprintf("aws://%s", a.Profile)
}

func (a *AWSConnection) Fetch() (keys chan data.Key, accounts chan data.Account) {
	keys = make(chan data.Key)
	accounts = make(chan data.Account)

	go func() {
	defer close(keys)
		defer close(accounts)

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
		output.Warn("Failed to find key pairs")
		return
	}

	bindings := make([]data.KeyBinding, 0)


		for _, p := range out.KeyPairs {
			fp := p.KeyFingerprint
			name := p.KeyName
			bindings = append(bindings, data.KeyBinding{KeyID: data.ID(*fp), Name: *name})
		}

		acct := data.Account{Type: "Account", Connection: a.Id(), Name: a.Profile, Keys: bindings}

		accounts <- acct

	}()

	return
}

func (a *AWSConnection) Id() data.ID {
	return data.ID(a.Profile)
}
