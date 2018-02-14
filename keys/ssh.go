package keys
import "strings"

type SSHPublicKey struct {
     Type, Content, Comment, Constraints string
}

func parseSshPublicKey(content string) Key {
     content = strings.Trim(content, " \t\n")
     command := ""

     if strings.HasPrefix(strings.ToLower(content), "command") {
       i := strings.Index(content, " ssh-")
       
       command=content[:i]
       content = strings.Trim(content[i:], " \t\n")
     }    

     slice := strings.SplitN(content, " ", 3)
     return SSHPublicKey{slice[0], slice[1], slice[2], command}
}
