// Package exec runs agent commands as real operating system processes.
package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"slices"
	"strings"
	"sync"
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
	return runner.run(argv, timeout, nil)
}

// RunEchoed is Run that also hands echo every line the command writes to
// stdout or to stderr, as soon as the line is complete. A line redrawn with
// carriage returns is handed over as it last read.
func (runner *CommandRunner) RunEchoed(argv []string, timeout time.Duration, echo func(line string)) (stdout string, stderr string, err error) {
	return runner.run(argv, timeout, echo)
}

func (runner *CommandRunner) run(argv []string, timeout time.Duration, echo func(line string)) (stdout string, stderr string, err error) {
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

	var stdoutBuffer, stderrBuffer strings.Builder
	command.Stdout = &stdoutBuffer
	command.Stderr = &stderrBuffer

	if echo != nil {
		// os/exec copies stdout and stderr on two goroutines, so echo is
		// called under one lock.
		var echoLock sync.Mutex
		lockedEcho := func(line string) {
			echoLock.Lock()
			defer echoLock.Unlock()
			echo(line)
		}
		stdoutLines := &lineWriter{echo: lockedEcho}
		stderrLines := &lineWriter{echo: lockedEcho}
		defer stdoutLines.flush()
		defer stderrLines.flush()
		command.Stdout = io.MultiWriter(&stdoutBuffer, stdoutLines)
		command.Stderr = io.MultiWriter(&stderrBuffer, stderrLines)
	}

	err = command.Run()
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

	return stdoutBuffer.String(), stderr, nil
}

// lineWriter hands every complete line written to it to echo, stripped of
// what a carriage return inside it has overwritten. Blank lines are dropped.
type lineWriter struct {
	echo    func(line string)
	pending []byte
}

func (writer *lineWriter) Write(chunk []byte) (int, error) {
	writer.pending = append(writer.pending, chunk...)
	for {
		end := bytes.IndexByte(writer.pending, '\n')
		if end < 0 {
			return len(chunk), nil
		}
		writer.emit(writer.pending[:end])
		writer.pending = writer.pending[end+1:]
	}
}

// flush hands over the last line when the command ended without a newline.
func (writer *lineWriter) flush() {
	writer.emit(writer.pending)
	writer.pending = nil
}

func (writer *lineWriter) emit(line []byte) {
	trimmed := bytes.TrimRight(line, "\r")
	if start := bytes.LastIndexByte(trimmed, '\r'); start >= 0 {
		trimmed = trimmed[start+1:]
	}
	if text := strings.TrimSpace(string(trimmed)); text != "" {
		writer.echo(text)
	}
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
