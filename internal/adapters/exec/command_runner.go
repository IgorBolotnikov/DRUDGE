// Package exec runs agent commands as real operating system processes.
package exec

import (
	"fmt"
	osexec "os/exec"
	"strings"
)

// CommandRunner is the os/exec backed implementation of the runner's
// CommandRunner port.
type CommandRunner struct{}

func NewCommandRunner() *CommandRunner {
	return &CommandRunner{}
}

// Run executes argv as a process and returns its stdout. No shell is involved,
// so an argument containing spaces, quotes or newlines reaches the command as
// written.
func (runner *CommandRunner) Run(argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("cannot run an empty command")
	}

	command := osexec.Command(argv[0], argv[1:]...)

	var stderr strings.Builder
	command.Stderr = &stderr

	stdout, err := command.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return "", fmt.Errorf("command %s failed: %w: %s", argv[0], err, message)
		}
		return "", fmt.Errorf("command %s failed: %w", argv[0], err)
	}

	return string(stdout), nil
}
