// Package runner contains an actual agent runner which implements tasks
package runner

import (
	"fmt"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

// TODO: add a new command `drg task run [task ID] which should run the Runner.RunTask()`

type RunnerService struct {
	logger *common.Logger
	cfg    *config.GlobalConfig
}

func New(logger *common.Logger, cfg *config.GlobalConfig) *RunnerService {
	return &RunnerService{
		logger: logger,
		cfg:    cfg,
	}
}

// RunTask runs a task until completion
func (service *RunnerService) RunTask(taskID task.TaskID) {
	// We need a mapping of commands which we should run in a subprocess
	// For each combination of env and harness
	// Step by step how the task is run
	// - change the task status to "in-progress"
	// - Load the task description into memory
	// - Load the special task implementation prompt (should be configurable in GlobalConfig)
	// - Create a worktree from default branch (configured in GlobalConfig)
	//   into local .drudge/worktrees/ dir.
	//   Name of the worktree should be `wt-[task-id]`
	//   cd into a worktree and checkout into a new
	//   branch `feat/[ticket ID if exists + /][slug-from-task-title]`
	prompt := "Implement the task"
	runnerID := "1"
	// run this command
	command := service.pickRunnerCommand(runnerID, prompt)
}

func (service *RunnerService) pickRunnerCommand(runnerID string, prompt string) string {
	name := formatRunnerName(runnerID, service.cfg.Runner.Harness)
	switch service.cfg.Runner.Env {
	case config.EnvDockerSbx:
		switch service.cfg.Runner.Harness {
		case config.HarnessClaudeCode:
			return fmt.Sprintf("sbx run --name %s -p %s", name, prompt)
		case config.HarnessOpencode:
			return fmt.Sprintf("sbx run --name %s -p %s", name, prompt)
		}
	}
	return ""
}

func formatRunnerName(runnerID string, harness config.Harness) string {
	switch harness {
	case config.HarnessClaudeCode:
		return fmt.Sprintf("drudge-claude-%s", runnerID)
	case config.HarnessOpencode:
		return fmt.Sprintf("drudge-opencode-%s", runnerID)
	}
	return fmt.Sprintf("drudge-unknown-%s", runnerID)
}
