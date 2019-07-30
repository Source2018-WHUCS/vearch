package util

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

type Command struct {
	Cmd  string
	Args []string
	Out  io.Writer
}

func (cmd *Command) Execute() error {
	aname, err := exec.LookPath(cmd.Cmd)
	if err != nil {
		aname, err = os.Getwd()
		aname = aname + "/" + cmd.Cmd
		if _, err = os.Stat(aname); err != nil {
			cmd.Out.Write([]byte("Can't find command path!"))
			return err
		}
	}

	command := exec.Cmd{
		Path: aname,
		Args: []string{cmd.Cmd},
	}
	if len(cmd.Args) > 0 {
		command.Args = append(command.Args, cmd.Args...)
	}

	stdout, _ := command.StdoutPipe()
	stderr, _ := command.StderrPipe()

	defer command.Wait()
	if err = command.Start(); err != nil {
		cmd.Out.Write([]byte(err.Error()))
	} else {
		stdout_ch, stderr_ch := make(chan []byte, 1024), make(chan []byte, 1024)
		go read(stdout, stdout_ch)
		go read(stderr, stderr_ch)

		for stdout_ch != nil || stderr_ch != nil {
			select {
			case stdout_buf, stdout_ok := <-stdout_ch:
				if stdout_ok {
					fmt.Println(string(stdout_buf))
					cmd.Out.Write(stdout_buf)
					if flusher, ok := cmd.Out.(http.Flusher); ok {
						flusher.Flush()
					}
				} else {
					stdout_ch = nil
				}
			case stderr_buf, stderr_ok := <-stderr_ch:
				if stderr_ok {
					fmt.Println(string(stderr_buf))
					cmd.Out.Write(stderr_buf)
					if flusher, ok := cmd.Out.(http.Flusher); ok {
						flusher.Flush()
					}
				} else {
					stderr_ch = nil
				}
			}
		}
	}
	return err
}

func read(r io.Reader, ch chan []byte) {
	br := bufio.NewReader(r)
	for {
		buf, err := br.ReadBytes('\n')
		if err == io.EOF {
			close(ch)
			return
		} else {
			ch <- buf
		}
	}
}
