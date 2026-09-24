// Package exec runs agent commands as real operating system processes.
package exec

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"slices"
	"strings"
	"time"
)

// pipeGrace is how long Run waits for a killed command's output pipes to
// close before it closes them itself. A command that spawns a background
// process hands it those pipes. The background process holds them open after
// the deadline kills the command, so they never reach EOF.
const pipeGrace = 100 * time.Millisecond

// CommandRunner is the os/exec backed implementation of the drudger's
// CommandRunner port.
type CommandRunner struct {
	// droppedEnv are the environment variables a command does not inherit.
	droppedEnv []string
}

func NewCommandRunner() *CommandRunner {
	return &CommandRunner{}
}

// WithoutEnv returns a runner whose commands do not inherit the named
// environment variables. The runner it is called on is left as it is.
func (runner *CommandRunner) WithoutEnv(names ...string) *CommandRunner {
	return &CommandRunner{droppedEnv: append(slices.Clone(runner.droppedEnv), names...)}
}

// environment is what a command starts with. A nil environment makes os/exec
// hand the command the environment of this process.
func (runner *CommandRunner) environment() []string {
	if len(runner.droppedEnv) == 0 {
		return nil
	}
	return slices.DeleteFunc(os.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return slices.Contains(runner.droppedEnv, name)
	})
}

// Run executes argv as a process and returns what it wrote to stdout and to
// stderr. A command can write to stderr and still succeed, meaning stderr comes
// back on both paths and the caller decides what it means.
//
// A command that runs longer than timeout is killed and the error wraps
// context.DeadlineExceeded. A timeout that is not positive is refused.
// TODO: Run makes its own context, so only the clock can stop a command. A TUI
// needs to cancel a long create on a keypress, which means the port takes a
// context and the caller owns the timeout.
func (runner *CommandRunner) Run(argv []string, timeout time.Duration) (stdout string, stderr string, err error) {
	if len(argv) == 0 {
		return "", "", fmt.Errorf("cannot run an empty command")
	}
	if timeout <= 0 {
		return "", "", fmt.Errorf("command %s needs a positive timeout, got %s", argv[0], timeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := osexec.CommandContext(ctx, argv[0], argv[1:]...)
	command.WaitDelay = pipeGrace
	command.Env = runner.environment()

	var stderrBuffer strings.Builder
	command.Stderr = &stderrBuffer

	stdoutBytes, err := command.Output()
	stderr = strings.TrimSpace(stderrBuffer.String())

	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", stderr, fmt.Errorf("command %s did not finish within %s and was killed: %w", strings.Join(argv, " "), timeout, context.DeadlineExceeded)
	}

	if err != nil {
		if stderr != "" {
			return "", stderr, fmt.Errorf("command %s failed: %w: %s", argv[0], err, stderr)
		}
		return "", stderr, fmt.Errorf("command %s failed: %w", argv[0], err)
	}

	return string(stdoutBytes), stderr, nil
}

// Start spawns argv and returns as soon as it is running. An agent runs for
// as long as its task takes without blocking this command.
func (runner *CommandRunner) Start(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("cannot start an empty command")
	}

	command := osexec.Command(argv[0], argv[1:]...)
	command.Env = runner.environment()

	if err := command.Start(); err != nil {
		return fmt.Errorf("command %s could not be started: %w", argv[0], err)
	}

	return command.Process.Release()
}
