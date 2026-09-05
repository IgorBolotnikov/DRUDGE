// Package runner contains an actual agent runner which implements tasks
package runner

import (
	"fmt"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

type RunnerService struct {
	logger    *common.Logger
	localCfg  *config.LocalConfig
	globalCfg *config.GlobalConfig
	tasks     *task.TaskService
	commands  CommandRunner
}

func New(logger *common.Logger, localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, tasks *task.TaskService, commands CommandRunner) *RunnerService {
	return &RunnerService{
		logger:    logger,
		localCfg:  localCfg,
		globalCfg: globalCfg,
		tasks:     tasks,
		commands:  commands,
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
	runnerName := formatRunnerName(projectSlug, runnerID, service.globalCfg.Runner.Harness)

	workspace, err := common.WorkDir()
	if err != nil {
		return fmt.Errorf("could not work out where to run task %s: %w", taskID, err)
	}
	runDir := common.RunDir(workspace, string(taskID))

	plan, err := service.pickRunnerCommand(projectSlug, runnerID, workspace, runDir)
	if err != nil {
		return err
	}

	if dryRun {
		service.logger.Info("Runner %d (%s) for task [%s] %s", runnerID, runnerName, taskToRun.ID, taskToRun.Title)
		service.logger.Info("Prompt (from %s):\n\n%s", promptSource, prompt)
		service.logger.Info("Commands:\n\n%s\n%s\n%s", formatArgv(plan.inspect), formatArgv(plan.create), formatArgv(plan.start))
		return nil
	}

	// TODO: before an agent is spawned, create a worktree for the task from the
	// default branch under the local worktrees dir, named wt-<task-id>, and
	// check out a branch named feat/<ticket-id>/<slug-from-task-title> in it.
	if err := service.ensureSandbox(plan, runnerName, workspace); err != nil {
		return err
	}

	if err := writeRunPrompt(runDir, prompt); err != nil {
		return err
	}

	// TODO: add a command that pings a task's runner session to tell whether it
	// is still alive, and frees the runner slot when it is not.
	if _, err := service.commands.Run(plan.start); err != nil {
		return fmt.Errorf("could not start runner %s for task %s: %w", runnerName, taskID, err)
	}

	taskToRun.Status = task.StatusInProgress
	taskToRun.StartedAt = time.Now().UTC()
	taskToRun.RunnerID = runnerID

	if err := service.tasks.UpdateTask(projectSlug, taskToRun); err != nil {
		return fmt.Errorf("runner %s is already working on task %s, but the task could not be marked as %q: %w", runnerName, taskID, task.StatusInProgress, err)
	}

	service.logger.Info("Runner %s is working on task [%s] %s", runnerName, taskToRun.ID, taskToRun.Title)
	service.logger.Info("Run directory: %s", runDir)
	return nil
}

// ensureSandbox creates the runner's sandbox unless it already exists.
// Creating one that is already there fails, so the listing decides there.
// An existing sandbox is only reused when it holds the workspace of this run.
func (service *RunnerService) ensureSandbox(plan sandboxPlan, runnerName, workspace string) error {
	listing, err := service.commands.Run(plan.inspect)
	if err != nil {
		return fmt.Errorf("could not list the sandboxes to look for %s: %w", runnerName, err)
	}

	existing, err := findSandbox(listing, runnerName)
	if err != nil {
		return err
	}
	if existing != nil {
		return checkSandboxWorkspace(existing, workspace)
	}

	if _, err := service.commands.Run(plan.create); err != nil {
		return fmt.Errorf("could not create sandbox %s: %w", runnerName, err)
	}
	return nil
}

// writeRunPrompt puts the rendered prompt in the run directory, where the
// agent reads it from inside its sandbox. It runs last, once the sandbox is
// known to be there, so a launch that never gets that far leaves no run
// directory for the status command to find.
func writeRunPrompt(runDir string, prompt string) error {
	if err := common.EnsureDir(runDir); err != nil {
		return err
	}
	return common.WriteFile(common.RunPromptPath(runDir), prompt)
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
