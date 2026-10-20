package drudger

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// SetupCommandStarted reports a launch running one setup command of a
// repository in the sandbox. It waits up to Timeout for the command to finish.
type SetupCommandStarted struct {
	Repository string
	Command    string
	Timeout    time.Duration
}

// SetupOutput reports one line a setup command wrote while it ran.
type SetupOutput struct {
	Line string
}

// exitCoder is an error that carries the exit code of a process.
type exitCoder interface {
	ExitCode() int
}

// runSetup runs the reset of a plan and then its setup commands in order, and
// stops at the first one that fails. Every line the setup commands write is
// reported and goes to the setup log of the run, which is rewritten after each
// command.
func (service *DrudgerService) runSetup(taskID task.TaskID, runDir string, reset sandboxCommand, commands []setupCommand) error {
	if _, _, err := service.runSbx(reset); err != nil {
		return fmt.Errorf("could not empty the PATH file and create the git hooks directories in the sandbox: %w", err)
	}

	logPath := common.RunSetupLogPath(runDir)
	var setupLog strings.Builder

	for _, setup := range commands {
		service.progress.Report(SetupCommandStarted{Repository: setup.repository, Command: setup.command, Timeout: setup.timeout})
		fmt.Fprintf(&setupLog, "$ %s  # in %s\n", setup.command, setup.repository)

		_, _, runErr := service.commands.RunEchoed(setup.argv, setup.timeout, func(line string) {
			setupLog.WriteString(line + "\n")
			service.progress.Report(SetupOutput{Line: line})
		})

		if err := service.runs.WriteSetupLog(taskID, setupLog.String()); err != nil {
			return fmt.Errorf("could not write the setup log of task %s to %s: %w", taskID, logPath, err)
		}
		if runErr != nil {
			return setupFailure(setup, runErr, logPath)
		}
	}
	return nil
}

// setupFailure explains a setup command that failed. A command killed by its
// timeout names the setting that raises it.
func setupFailure(setup setupCommand, err error, logPath string) error {
	failed := fmt.Sprintf("setup command %q of repository %s", setup.command, setup.repository)

	if timedOut(err) {
		return fmt.Errorf(
			"%s did not finish within %s and was killed, raise %q in the global config if it needs longer, its output is in %s: %w",
			failed, setup.timeout, SetupTimeoutKey, logPath, err,
		)
	}

	var exited exitCoder
	if errors.As(err, &exited) {
		return fmt.Errorf("%s exited with code %d, its output is in %s: %w", failed, exited.ExitCode(), logPath, err)
	}
	return fmt.Errorf("%s failed, its output is in %s: %w", failed, logPath, err)
}
