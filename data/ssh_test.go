package data

import (
	"bufio"
	"crypto/sha1"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/deweysasser/pkcs8"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func checke(t *testing.T, e error) {
	if e != nil {
		t.Error(e)
	}
}

func assertStringsEquals(t *testing.T, s1, s2 string) {
	if s1 != s2 {
		t.Logf("Expected [%s] but got [%s]", s1, s2)
		t.Fail()
	}
}

func TestPublicKey(t *testing.T) {
	data := "AAAAB3NzaC1yc2EAAAADAQABAAABAQDEhoo9i/AwdwWx2xFcQjZkQxlNlex1p7pyOn7qitncnc/+bEHSARGoflqMMFgoBMrsKcQUZXt+LpBvlwGbTqATfat5SwKJbQi2EcoRr8j0e1gsG357zv0i/GuemdTctyk2Hdxq+MkuSlSMlswoAPLfGhFBUiBNLIrb5wwK8MNJjpRkqONxtDQHYpeZ7J+PdSVAQYJ6aNxrA5zRd732CHDyMkHIvnmb+vFa7rPYYwLyzborMrTEQXc1IpqNOzkF33AXAmqsjwNabmReRyerVGZ5cyLJEhn0Yjkixa1lt4RcioV8y4OnLXeHOB7DP1HEko3Ox8Tc16r+b2v70+YBc2c5"
	bKey, e := base64.StdEncoding.DecodeString(data)
	checke(t, e)

	sshKey, e := ssh.ParsePublicKey(bKey)
	checke(t, e)
	p := PublicKey{sshKey}

	b, e := json.Marshal(&p)
	checke(t, e)

	assertStringsEquals(t, `{"Type":"ssh-rsa","Data":"AAAAB3NzaC1yc2EAAAADAQABAAABAQDEhoo9i/AwdwWx2xFcQjZkQxlNlex1p7pyOn7qitncnc/+bEHSARGoflqMMFgoBMrsKcQUZXt+LpBvlwGbTqATfat5SwKJbQi2EcoRr8j0e1gsG357zv0i/GuemdTctyk2Hdxq+MkuSlSMlswoAPLfGhFBUiBNLIrb5wwK8MNJjpRkqONxtDQHYpeZ7J+PdSVAQYJ6aNxrA5zRd732CHDyMkHIvnmb+vFa7rPYYwLyzborMrTEQXc1IpqNOzkF33AXAmqsjwNabmReRyerVGZ5cyLJEhn0Yjkixa1lt4RcioV8y4OnLXeHOB7DP1HEko3Ox8Tc16r+b2v70+YBc2c5"}`, string(b))

	p = PublicKey{}
	e = json.Unmarshal(b, &p)
	checke(t, e)

	assertStringsEquals(t, "ssh-rsa", p.Key.Type())

}
func assertTrue(t *testing.T, message string, b bool) {
	if !b {
		t.Error(message)
	}
}

func TestSSHPublicKeyParsing(t *testing.T) {
	path := "test-data/public-keys"
	keys, err := ioutil.ReadDir(path)
	checke(t, err)

	for _, key := range keys {
		kp := path + "/" + key.Name()
		t.Run(key.Name(), func(t *testing.T) {
			k := Read(kp)
			if k == nil {
				t.Error("Failed to parse " + kp)
			}
		})
	}
}

func SkipTestSSHPrivateKeyParse(t *testing.T) {
	t.Run("RSA", func(t *testing.T) {
		key := Read("test-data/rsa")
		if key == nil {
			t.Error("Failed to parse RSA")
		}
		rsa := key.(*SSHKey)
		assertStringsEquals(t, "ssh-rsa", rsa.KeyType())
		assertStringsEquals(t, "AAAAB3NzaC1yc2EAAAADAQABAAABAQDEhoo9i/AwdwWx2xFcQjZkQxlNlex1p7pyOn7qitncnc/+bEHSARGoflqMMFgoBMrsKcQUZXt+LpBvlwGbTqATfat5SwKJbQi2EcoRr8j0e1gsG357zv0i/GuemdTctyk2Hdxq+MkuSlSMlswoAPLfGhFBUiBNLIrb5wwK8MNJjpRkqONxtDQHYpeZ7J+PdSVAQYJ6aNxrA5zRd732CHDyMkHIvnmb+vFa7rPYYwLyzborMrTEQXc1IpqNOzkF33AXAmqsjwNabmReRyerVGZ5cyLJEhn0Yjkixa1lt4RcioV8y4OnLXeHOB7DP1HEko3Ox8Tc16r+b2v70+YBc2c5", rsa.PublicKeyString())
	})

	t.Run("DSS", func(t *testing.T) {
		key := Read("test-data/dss")
		if key == nil {
			t.Error("Failed to parse DSS")
		}
		dss := key.(*SSHKey)
		assertStringsEquals(t, "ssh-dss", dss.KeyType())
		assertStringsEquals(t, "AAAAB3NzaC1kc3MAAACBAMZhAjMPsL/oo9RZiD7jfWBOVGoLqwdwtjuTkaKVFmBVBh+c2nMi11zVzRz1JqbXR15QNyaDc2EumZTC2WTyas4uSXTh2F6Ohto+a2QnCN3rjsiBsXHnr6hbBN+Qs8uJ/+ssGDpsWKIpWOL3+Q6QmHQZg+df4XtBlMyehCWr7jCdAAAAFQCrynAE+Z6tGteawaHWa8ReOpYkrQAAAIB3cd1Ls/1ox/gNNMqTbuAvWQIgIda7Uw+OHU55EyeryPR9e2GH6rsHWCwd47cyurOukqF+e5FH/dnj7K/Kt4BFXPeR0YU4KaiAZIEl8I7Kcdazxz3vWgK3sTKRy10ABqEZL9oUazMfX43IaiPeiU6nwgrMHokTwKLkZH+iBwN8JQAAAIEAo+h6Lop9my2BxrHKSmhQfya3rl0N35ZDk/8kExLW1xkpQmzARrCMrw3YNuRCNgrh5Ds7EdyG0HyjWnnSnPBXqCxFfDTtaGeieLquocEK3M5DGckgI4IEa9pvL3fVZ/cHT3YxC369PF/vX9l7TPHF6Au8lnEFEzNyZLQvsfrqxgg=", dss.PublicKeyString())
	})
}

func TestSSHPublicKeyParse(t *testing.T) {
	t.Run("RSA", func(t *testing.T) {
		key := Read("test-data/rsa.pub")
		if key == nil {
			t.Error("Failed to parse RSA")
		}
		rsa := key.(*SSHKey)
		assertStringsEquals(t, "ssh-rsa", rsa.KeyType())
		assertStringsEquals(t, "SHA256:mbhMXOdSermDODXkg5fBUQN9yst7W9Fkn9yurscQSOQ", string(rsa.Id()))
		assertStringsEquals(t, "ca:c1:67:18:a3:79:a5:46:03:8b:3e:a1:67:4b:8e:39", string(rsa.Identifiers()[1]))
		assertTrue(t, "correct comment", rsa.Comments.Contains("dewey@FlynnRyder"))
		assertStringsEquals(t, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDEhoo9i/AwdwWx2xFcQjZkQxlNlex1p7pyOn7qitncnc/+bEHSARGoflqMMFgoBMrsKcQUZXt+LpBvlwGbTqATfat5SwKJbQi2EcoRr8j0e1gsG357zv0i/GuemdTctyk2Hdxq+MkuSlSMlswoAPLfGhFBUiBNLIrb5wwK8MNJjpRkqONxtDQHYpeZ7J+PdSVAQYJ6aNxrA5zRd732CHDyMkHIvnmb+vFa7rPYYwLyzborMrTEQXc1IpqNOzkF33AXAmqsjwNabmReRyerVGZ5cyLJEhn0Yjkixa1lt4RcioV8y4OnLXeHOB7DP1HEko3Ox8Tc16r+b2v70+YBc2c5\n", rsa.PublicKeyString())
	})

	t.Run("DSS", func(t *testing.T) {
		key := Read("test-data/dss.pub")
		if key == nil {
			t.Error("Failed to parse DSS")
		}
		dss := key.(*SSHKey)
		assertStringsEquals(t, "ssh-dss", dss.KeyType())
		assertTrue(t, "correct comment", dss.Comments.Contains("dewey@FlynnRyder"))
		assertStringsEquals(t, "ssh-dss AAAAB3NzaC1kc3MAAACBAMZhAjMPsL/oo9RZiD7jfWBOVGoLqwdwtjuTkaKVFmBVBh+c2nMi11zVzRz1JqbXR15QNyaDc2EumZTC2WTyas4uSXTh2F6Ohto+a2QnCN3rjsiBsXHnr6hbBN+Qs8uJ/+ssGDpsWKIpWOL3+Q6QmHQZg+df4XtBlMyehCWr7jCdAAAAFQCrynAE+Z6tGteawaHWa8ReOpYkrQAAAIB3cd1Ls/1ox/gNNMqTbuAvWQIgIda7Uw+OHU55EyeryPR9e2GH6rsHWCwd47cyurOukqF+e5FH/dnj7K/Kt4BFXPeR0YU4KaiAZIEl8I7Kcdazxz3vWgK3sTKRy10ABqEZL9oUazMfX43IaiPeiU6nwgrMHokTwKLkZH+iBwN8JQAAAIEAo+h6Lop9my2BxrHKSmhQfya3rl0N35ZDk/8kExLW1xkpQmzARrCMrw3YNuRCNgrh5Ds7EdyG0HyjWnnSnPBXqCxFfDTtaGeieLquocEK3M5DGckgI4IEa9pvL3fVZ/cHT3YxC369PF/vX9l7TPHF6Au8lnEFEzNyZLQvsfrqxgg=\n", dss.PublicKeyString())
	})
}

func TestReadAuthorizedKeys(t *testing.T) {
	file, err := os.Open("test-data/authorized_keys")
	checke(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Constrained Key is the first line
	scanner.Scan()
	text := scanner.Text()

	key := NewKey(text, time.Time{})

	if key == nil {
		t.Errorf("Key is nil")
	}

	sshkey := key.(*SSHKey)

	assertStringsEquals(t, "ssh-rsa", sshkey.KeyType())
	assertStringsEquals(t, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCpVaCpLlQ8Wf4KgcRwsIvXCvf0Onkp9hZ2Sov5s2ZiIqJne8Rk9kx3CoSHGMpCSCBuGybs8k/8ga7g/l6+bKDc3aDGWw52+7ClBGz4xjL5C9HXub2iKRdxIesDtkQtQMawFobBTi9hiW92SoK1H/AmLhHDxicfidXPaOcNY57PWZDqEmR2PWo0k4oNn0zxQO3UJmfiKNoR6ozJ3JDWGCu2SMh/YobKwNSlge6YsVKO4zpxR3wBbHS9CYL2xE6QMyN1KnJ+ACoeZF8tkXThOAgH5VERoM+KawAHK0Hqpqh8d85jQU7ul9ernFCip2zVAC/hsobORmHGyvGd9aWDXZTB\n", sshkey.PublicKeyString())
	assertTrue(t, "constrained key", sshkey.Comments.Contains("constrained key"))
	//	assertStringsEquals(t, "command=\"/bin/ps -ef\",no-port-forwarding,no-X11-forwarding,no-pty", sshkey.Constraints)
}

func TestSSHId(t *testing.T) {
	key := Read("test-data/rsa.pub")
	assertStringsEquals(t, "SHA256:mbhMXOdSermDODXkg5fBUQN9yst7W9Fkn9yurscQSOQ", string(key.Id()))
}

func TestSSHJSon(t *testing.T) {
	expected := `{
  "Type": "SSHKey",
  "Names": [
    "test name"
  ],
  "Replacement": "other id",
  "Earliest": "0001-01-01T00:00:00Z",
  "Ids": [
    "SHA256:mbhMXOdSermDODXkg5fBUQN9yst7W9Fkn9yurscQSOQ",
    "ca:c1:67:18:a3:79:a5:46:03:8b:3e:a1:67:4b:8e:39"
  ],
  "PublicKey": {
    "Type": "ssh-rsa",
    "Data": "AAAAB3NzaC1yc2EAAAADAQABAAABAQDEhoo9i/AwdwWx2xFcQjZkQxlNlex1p7pyOn7qitncnc/+bEHSARGoflqMMFgoBMrsKcQUZXt+LpBvlwGbTqATfat5SwKJbQi2EcoRr8j0e1gsG357zv0i/GuemdTctyk2Hdxq+MkuSlSMlswoAPLfGhFBUiBNLIrb5wwK8MNJjpRkqONxtDQHYpeZ7J+PdSVAQYJ6aNxrA5zRd732CHDyMkHIvnmb+vFa7rPYYwLyzborMrTEQXc1IpqNOzkF33AXAmqsjwNabmReRyerVGZ5cyLJEhn0Yjkixa1lt4RcioV8y4OnLXeHOB7DP1HEko3Ox8Tc16r+b2v70+YBc2c5"
  },
  "Comments": [
    "dewey@FlynnRyder"
  ]
}`

	key := Read("test-data/rsa.pub")
	// Override the first notice time so the test can pass consistently
	key.(*SSHKey).Earliest = time.Time{}

	key.(*SSHKey).Names.Add("test name")
	key.(*SSHKey).Replacement = "other id"

	sJson, error := json.MarshalIndent(key, "", "  ")
	checke(t, error)
	ioutil.WriteFile("temp", sJson, 0666)
	assertStringsEquals(t, expected, string(sJson))

	var newkey Key = new(SSHKey)
	e := json.Unmarshal([]byte(expected), &newkey)
	checke(t, e)

	fmt.Printf("Recovered key is %v\n", newkey)

	assertStringsEquals(t, string(key.Id()), string(newkey.Id()))
	assertStringsEquals(t, string(key.ReplacementID()), string(newkey.ReplacementID()))
	assertTrue(t, "Correct Name", newkey.(*SSHKey).Names.Contains("test name"))

	assertTrue(t, "Correct comment", newkey.(*SSHKey).Comments.Contains("dewey@FlynnRyder"))

	sJson2, err := json.MarshalIndent(newkey, "", "  ")
	checke(t, err)
	assertStringsEquals(t, expected, string(sJson2))
}

func TestPublicKeyFingerprints(t *testing.T) {
	k2 := Read("test-data/rsa.pub")

	if k2 == nil {
		t.Error("Failed to parse locally generated SSH key")
	}

	// TODO:  resurrect AWS public key fingerprints
	// This is the Amazon generated fingerprint for test-data/rsa
	//s2 := k2.(*SSHKey)
	//if !s2.Ids.Contains(ID("bb:ee:0f:90:22:18:13:a0:40:e5:cc:67:81:1b:4b:6c")) {
	//	fmt.Println("Keys: ", s2.Ids)
	//	t.Error("Failed to contain AWS public ID")
	//}
}

func TestPrivateKeyParsing(t *testing.T) {
	key := Read("test-data/locksmith-test-aws-generated.pem")
	if bytes, err := json.MarshalIndent(key, "", "  "); err == nil {
		ioutil.WriteFile("locksmith-test-aws-generated.json", bytes, 666)
	} else {
		t.Error("Failed to write key json file")
	}

	if key == nil {
		t.Error("Failed to parse RSA key")
	}

	//s := key.(*SSHKey)

	//if len(s.Ids.Ids) <1 {
	//	t.Error("No IDs from private key")
	//}

	//if !s.Ids.Contains(ID("6a:49:68:aa:d2:29:b2:e3:be:86:4a:6b:5f:e7:b6:fd:c8:7b:ad:3b")) {
	//	t.Error("Failed to contain AWS private ID")
	//}

}

func skipTestAWSIds(t *testing.T) {
	if bytes, err := ioutil.ReadFile("test-data/locksmith-test-aws-generated.pem"); err == nil {
		block, _ := pem.Decode(bytes)
		fmt.Println("PEM decode bytes in", block.Type, "are", block.Bytes)
		data := make(map[string]interface{})
		if _, err := asn1.Unmarshal(block.Bytes, data); err == nil {
			fmt.Println("ASN structure is ", data)
		}
		if pk, err := ssh.ParseRawPrivateKey(bytes); err == nil {
			if bytes, err := pkcs8.ConvertPrivateKeyToPKCS8(pk); err == nil {
				fmt.Println("converted bytes are", bytes)
				id := ID(asHex(sha1.Sum(bytes)))
				assertStringsEquals(t, "6a:49:68:aa:d2:29:b2:e3:be:86:4a:6b:5f:e7:b6:fd:c8:7b:ad:3b", string(id))
				return
			}
		}
	}
}

func TestNewSSHKeyFromFingerprint(t *testing.T) {
	key := NewSSHKeyFromFingerprint("testing", time.Time{}, "id1")

	if bytes, e := json.Marshal(key); e != nil {
		t.Error("Failed to marshal: ", e)
	} else {
		key2 := SSHKey{}
		if e := json.Unmarshal(bytes, &key2); e != nil {
			t.Error("Failed to unmarshal: ", e)
		} else {
			assertStringsEquals(t, "id1", string(key2.Id()))
			assertStringsEquals(t, "testing", key2.Names.StringArray()[0])
		}
	}
}
