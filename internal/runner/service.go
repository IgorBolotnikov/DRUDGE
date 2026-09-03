// Package runner contains an actual agent runner which implements tasks
package runner

import (
	"fmt"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

type RunnerService struct {
	logger *common.Logger
	cfg    *config.GlobalConfig
	tasks  *task.TaskService
}

func New(logger *common.Logger, cfg *config.GlobalConfig, tasks *task.TaskService) *RunnerService {
	return &RunnerService{
		logger: logger,
		cfg:    cfg,
		tasks:  tasks,
	}
}

// RunTask hands one task to an agent. In dry run mode it only resolves and
// prints what the agent would be given, and writes nothing.
func (service *RunnerService) RunTask(projectSlug string, taskID task.TaskID, dryRun bool) error {
	taskToRun, err := service.tasks.GetTask(projectSlug, taskID)
	if err != nil {
		return err
	}

	if taskToRun.Status != task.StatusTodo {
		return fmt.Errorf("task %s is %q, only %q tasks can be run", taskID, taskToRun.Status, task.StatusTodo)
	}

	prompt, err := renderPrompt(defaultPromptTemplate, taskToRun)
	if err != nil {
		return err
	}

	if dryRun {
		service.logger.Info("Prompt for task [%s] %s:\n\n%s", taskToRun.ID, taskToRun.Title, prompt)
		return nil
	}

	// TODO: before an agent is spawned, create a worktree for the task from the
	// default branch under the local worktrees dir, named wt-<task-id>, and
	// check out a branch named feat/<ticket-id>/<slug-from-task-title> in it.
	return fmt.Errorf("spawning an agent is not implemented yet, pass --dry-run to preview the prompt")
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
