// Package runner contains an actual agent runner which implements tasks
package runner

import (
	"fmt"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

type RunnerService struct {
	logger    *common.Logger
	localCfg  *config.LocalConfig
	globalCfg *config.GlobalConfig
	tasks     *task.TaskService
}

func New(logger *common.Logger, localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, tasks *task.TaskService) *RunnerService {
	return &RunnerService{
		logger:    logger,
		localCfg:  localCfg,
		globalCfg: globalCfg,
		tasks:     tasks,
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

	promptTemplate, promptSource, err := resolvePromptTemplate(service.localCfg, service.globalCfg)
	if err != nil {
		return err
	}

	prompt, err := renderPrompt(promptTemplate, taskToRun)
	if err != nil {
		return fmt.Errorf("%s: %w", promptSource, err)
	}

	runnerID, err := service.allocateRunnerID(projectSlug)
	if err != nil {
		return err
	}
	runnerName := formatRunnerName(runnerID, service.globalCfg.Runner.Harness)

	if dryRun {
		service.logger.Info("Runner %d (%s) for task [%s] %s", runnerID, runnerName, taskToRun.ID, taskToRun.Title)
		service.logger.Info("Prompt (from %s):\n\n%s", promptSource, prompt)
		return nil
	}

	// TODO: before an agent is spawned, create a worktree for the task from the
	// default branch under the local worktrees dir, named wt-<task-id>, and
	// check out a branch named feat/<ticket-id>/<slug-from-task-title> in it.
	return fmt.Errorf("spawning an agent is not implemented yet, pass --dry-run to preview the prompt")
}

// allocateRunnerID picks the lowest free runner slot of a project's pool. A
// slot is taken for as long as the task holding it is in progress, so the pool
// is recomputed from the project's tasks on every run.
func (service *RunnerService) allocateRunnerID(projectSlug string) (int, error) {
	tasks, err := service.tasks.ListTasks(projectSlug)
	if err != nil {
		return 0, fmt.Errorf("could not read the project's tasks to work out which runners are busy: %w", err)
	}

	occupied := occupiedRunnerIDs(tasks)
	limit := config.ResolveMaxConcurrentRunners(service.localCfg, service.globalCfg)

	for candidate := 1; candidate <= limit; candidate++ {
		if !occupied[candidate] {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("all %d runners of project %s are busy with %q tasks, wait for one to finish or raise %s in the config", limit, projectSlug, task.StatusInProgress, config.MaxConcurrentRunnersKey)
}

// occupiedRunnerIDs collects the runner slots held by in-progress tasks.
func occupiedRunnerIDs(tasks []*task.Task) map[int]bool {
	occupied := make(map[int]bool)
	for _, candidate := range tasks {
		if candidate.Status == task.StatusInProgress && candidate.RunnerID > 0 {
			occupied[candidate.RunnerID] = true
		}
	}
	return occupied
}

func (service *RunnerService) pickRunnerCommand(runnerID int, prompt string) string {
	name := formatRunnerName(runnerID, service.globalCfg.Runner.Harness)
	switch service.globalCfg.Runner.Env {
	case config.EnvDockerSbx:
		switch service.globalCfg.Runner.Harness {
		case config.HarnessClaudeCode:
			return fmt.Sprintf("sbx run --name %s -p %s", name, prompt)
		case config.HarnessOpencode:
			return fmt.Sprintf("sbx run --name %s -p %s", name, prompt)
		}
	}
	return ""
}

func formatRunnerName(runnerID int, harness config.Harness) string {
	switch harness {
	case config.HarnessClaudeCode:
		return fmt.Sprintf("drudge-claude-%d", runnerID)
	case config.HarnessOpencode:
		return fmt.Sprintf("drudge-opencode-%d", runnerID)
	}
	return fmt.Sprintf("drudge-unknown-%d", runnerID)
}
