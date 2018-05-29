package connection

import (
	"os/exec"
	"fmt"
	"io"
	"github.com/deweysasser/locksmith/output"
	"bufio"
	"strings"
	"errors"
)

type SshCmd struct {
	cmd *exec.Cmd
	stdin io.WriteCloser
	stdout, stderr *bufio.Reader
}

func NewSshCmd(host string) (*SshCmd, error){
	scmd := &SshCmd{}

	output.Debug(fmt.Sprintf("Running SSH cmd: ssh %s", host))

	scmd.cmd = exec.Command("ssh", host)


	var err error
	if scmd.stdin, err = scmd.cmd.StdinPipe(); err != nil {
		return nil, err
	}

	if stdout, err := scmd.cmd.StdoutPipe(); err != nil {
		return nil, err
	} else {
		scmd.stdout = bufio.NewReader(stdout)
	}

	if stderr, err := scmd.cmd.StderrPipe(); err != nil {
		return nil, err
	} else {
		scmd.stderr = bufio.NewReader(stderr)
	}


	if err := scmd.cmd.Start(); err != nil {
		return nil, err
	}

	// to discard any banner message
	scmd.Run("true")

	return scmd, nil
}

// TODO:  implement a timeout
func (s *SshCmd) Run(cmd string) (string, error) {
	boundary := "-----cmd bundary-----"

	output.Debug("Running", cmd)
	s.stdin.Write([]byte(fmt.Sprintf("%s\n", cmd)))
	s.stdin.Write([]byte(fmt.Sprintf("echo %s: $?\n", boundary)))

	var result []string
	for {
		if bytes, _, err := s.stdout.ReadLine(); err == nil {
			sBytes := string(bytes)
			//output.Debug(fmt.Sprintf("Read from %s: [%s]",cmd, sBytes))
			if strings.HasPrefix(sBytes, boundary) {
				//output.Debug("Prefix found")
				var exitVal int
				if _, err := fmt.Sscan(sBytes[(len(boundary)+2):], &exitVal); err != nil {
					return "", errors.New("Failed to convert exit value to integer in " + sBytes)
				}

				output.Debug("Exit val is ", exitVal)
				if exitVal > 0 {
					return strings.Join(result, "\n"), errors.New(fmt.Sprint("Non-zero exit: ", exitVal))
				} else {
					return strings.Join(result, "\n"), nil
				}
				output.Debug("unreachable")
			}
			result = append(result, string(bytes))
		} else {
			return "", err
		}
	}
}

func (s *SshCmd) Close() {
	s.stdin.Write([]byte("exit\n"))
	s.stdin.Close()
	// TODO:  add a timeout to this
	s.cmd.Wait()
}