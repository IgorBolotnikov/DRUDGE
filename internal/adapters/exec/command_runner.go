// Package exec runs agent commands as real operating system processes.
package exec

import (
	"fmt"
	osexec "os/exec"
	"strings"
)

// CommandRunner is the os/exec backed implementation of the drudger's
// CommandRunner port.
type CommandRunner struct{}

func NewCommandRunner() *CommandRunner {
	return &CommandRunner{}
}

// Run executes argv as a process and returns what it wrote to stdout and to
// stderr. A command can write to stderr and still succeed, so stderr comes
// back on both paths and the caller decides what it means.
func (runner *CommandRunner) Run(argv []string) (string, string, error) {
	if len(argv) == 0 {
		return "", "", fmt.Errorf("cannot run an empty command")
	}

	command := osexec.Command(argv[0], argv[1:]...)

	var stderrBuffer strings.Builder
	command.Stderr = &stderrBuffer

	stdout, err := command.Output()
	stderr := strings.TrimSpace(stderrBuffer.String())
	if err != nil {
		if stderr != "" {
			return "", stderr, fmt.Errorf("command %s failed: %w: %s", argv[0], err, stderr)
		}
		return "", stderr, fmt.Errorf("command %s failed: %w", argv[0], err)
	}

	return string(stdout), stderr, nil
}

// Start spawns argv and returns as soon as it is running. An agent runs for
// as long as its task takes without blocking this command.
func (runner *CommandRunner) Start(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("cannot start an empty command")
	}

	command := osexec.Command(argv[0], argv[1:]...)

	if err := command.Start(); err != nil {
		return fmt.Errorf("command %s could not be started: %w", argv[0], err)
	}

	return command.Process.Release()
}
