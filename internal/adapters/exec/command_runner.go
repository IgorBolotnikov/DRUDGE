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

// Run executes argv as a process and returns its stdout.
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

// Start spawns argv and returns as soon as it is running, leaving it to finish
// on its own. An agent runs for as long as its task takes, and drudge exits
// long before that.
//
// The command inherits none of drudge's streams. Stdout on a pipe would hold
// whoever reads drudge's output until the agent finished, which is the stall
// this avoids. Losing the command's own stderr is the price. What the agent
// did is recorded in its run directory, which is where drudge looks anyway.
func (runner *CommandRunner) Start(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("cannot start an empty command")
	}

	command := osexec.Command(argv[0], argv[1:]...)

	if err := command.Start(); err != nil {
		return fmt.Errorf("command %s could not be started: %w", argv[0], err)
	}

	// Nothing waits for this process, so drudge lets go of it and leaves the
	// operating system to clean it up.
	return command.Process.Release()
}
