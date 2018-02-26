package keys

import (
	"bufio"
//	"golang.org/x/crypto/ssh"
//	"io/ioutil"
	"os"
//	"fmt"
	"testing"
)

func assertStringsEquals(t *testing.T, s1, s2 string) {
	if s1 != s2 {
		t.Logf("Expected [%s] but got [%s]", s1, s2)
		t.Fail()
	}
}

func TestSSHPublicKeyParse(t *testing.T) {
	t.Run("RSA", func(t *testing.T) {
		key := Read("test-data/rsa.pub")
		if key == nil {
			t.Fatal("Failed to parse RSA")
		}
		rsa := key.(*SSHPublicKey)
		assertStringsEquals(t, "ssh-rsa", rsa.Type)
		assertStringsEquals(t, "dewey@FlynnRyder", rsa.Comment)
		assertStringsEquals(t, "AAAAB3NzaC1yc2EAAAADAQABAAABAQDEhoo9i/AwdwWx2xFcQjZkQxlNlex1p7pyOn7qitncnc/+bEHSARGoflqMMFgoBMrsKcQUZXt+LpBvlwGbTqATfat5SwKJbQi2EcoRr8j0e1gsG357zv0i/GuemdTctyk2Hdxq+MkuSlSMlswoAPLfGhFBUiBNLIrb5wwK8MNJjpRkqONxtDQHYpeZ7J+PdSVAQYJ6aNxrA5zRd732CHDyMkHIvnmb+vFa7rPYYwLyzborMrTEQXc1IpqNOzkF33AXAmqsjwNabmReRyerVGZ5cyLJEhn0Yjkixa1lt4RcioV8y4OnLXeHOB7DP1HEko3Ox8Tc16r+b2v70+YBc2c5", rsa.Content)
	})

	t.Run("DSS", func(t *testing.T) {
		key := Read("test-data/dss.pub")
		if key == nil {
			t.Fatal("Failed to parse DSS")
		}
		dss := key.(*SSHPublicKey)
		assertStringsEquals(t, "ssh-dss", dss.Type)
		assertStringsEquals(t, "dewey@FlynnRyder", dss.Comment)
		assertStringsEquals(t, "AAAAB3NzaC1kc3MAAACBAMZhAjMPsL/oo9RZiD7jfWBOVGoLqwdwtjuTkaKVFmBVBh+c2nMi11zVzRz1JqbXR15QNyaDc2EumZTC2WTyas4uSXTh2F6Ohto+a2QnCN3rjsiBsXHnr6hbBN+Qs8uJ/+ssGDpsWKIpWOL3+Q6QmHQZg+df4XtBlMyehCWr7jCdAAAAFQCrynAE+Z6tGteawaHWa8ReOpYkrQAAAIB3cd1Ls/1ox/gNNMqTbuAvWQIgIda7Uw+OHU55EyeryPR9e2GH6rsHWCwd47cyurOukqF+e5FH/dnj7K/Kt4BFXPeR0YU4KaiAZIEl8I7Kcdazxz3vWgK3sTKRy10ABqEZL9oUazMfX43IaiPeiU6nwgrMHokTwKLkZH+iBwN8JQAAAIEAo+h6Lop9my2BxrHKSmhQfya3rl0N35ZDk/8kExLW1xkpQmzARrCMrw3YNuRCNgrh5Ds7EdyG0HyjWnnSnPBXqCxFfDTtaGeieLquocEK3M5DGckgI4IEa9pvL3fVZ/cHT3YxC369PF/vX9l7TPHF6Au8lnEFEzNyZLQvsfrqxgg=", dss.Content)
	})
}

func TestReadAuthorizedKeys(t *testing.T) {
	file, err := os.Open("test-data/authorized_keys")
	check(err)
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Constrained key is the first line
	scanner.Scan()
	text := scanner.Text()

	key := New(text)

	if key == nil {
		t.Fatalf("Key is nil")
	}

	sshkey := key.(*SSHPublicKey)

	assertStringsEquals(t, "ssh-rsa", sshkey.Type)
	assertStringsEquals(t, "AAAAB3NzaC1yc2EAAAADAQABAAABAQCpVaCpLlQ8Wf4KgcRwsIvXCvf0Onkp9hZ2Sov5s2ZiIqJne8Rk9kx3CoSHGMpCSCBuGybs8k/8ga7g/l6+bKDc3aDGWw52+7ClBGz4xjL5C9HXub2iKRdxIesDtkQtQMawFobBTi9hiW92SoK1H/AmLhHDxicfidXPaOcNY57PWZDqEmR2PWo0k4oNn0zxQO3UJmfiKNoR6ozJ3JDWGCu2SMh/YobKwNSlge6YsVKO4zpxR3wBbHS9CYL2xE6QMyN1KnJ+ACoeZF8tkXThOAgH5VERoM+KawAHK0Hqpqh8d85jQU7ul9ernFCip2zVAC/hsobORmHGyvGd9aWDXZTB", sshkey.Content)
	assertStringsEquals(t, "constrained key", sshkey.Comment)
	assertStringsEquals(t, "command=\"/bin/ps -ef\",no-port-forwarding,no-X11-forwarding,no-pty", sshkey.Constraints)
}

func TestSSHId(t *testing.T) {
	key := Read("test-data/rsa.pub")
	assertStringsEquals(t, "SHA256:mbhMXOdSermDODXkg5fBUQN9yst7W9Fkn9yurscQSOQ", key.Id())
}

func TestSSHJSon(t *testing.T) {
	key := Read("test-data/rsa.pub")

	json, error := key.Json()
	check(error)
	assertStringsEquals(t, `{"Type":"ssh-rsa","Content":"AAAAB3NzaC1yc2EAAAADAQABAAABAQDEhoo9i/AwdwWx2xFcQjZkQxlNlex1p7pyOn7qitncnc/+bEHSARGoflqMMFgoBMrsKcQUZXt+LpBvlwGbTqATfat5SwKJbQi2EcoRr8j0e1gsG357zv0i/GuemdTctyk2Hdxq+MkuSlSMlswoAPLfGhFBUiBNLIrb5wwK8MNJjpRkqONxtDQHYpeZ7J+PdSVAQYJ6aNxrA5zRd732CHDyMkHIvnmb+vFa7rPYYwLyzborMrTEQXc1IpqNOzkF33AXAmqsjwNabmReRyerVGZ5cyLJEhn0Yjkixa1lt4RcioV8y4OnLXeHOB7DP1HEko3Ox8Tc16r+b2v70+YBc2c5","Comment":"dewey@FlynnRyder","Constraints":""}`, string(json))

	newkey := LoadJson(json)

	if key.Id() != newkey.Id() {
		t.Fatalf("Failed to load JSON")
	}
}


