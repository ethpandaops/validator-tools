package deposit

import (
	"os/exec"
)

type Command struct {
	*exec.Cmd
}

func NewCommand(name string, args []string) *Command {
	return &Command{
		Cmd: exec.Command(name, args...),
	}
}
