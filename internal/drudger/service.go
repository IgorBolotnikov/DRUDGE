// Package drudger hands tasks to agents working in sandboxes
package drudger

import (
	"fmt"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

type DrudgerService struct {
	logger    *common.Logger
	localCfg  *config.LocalConfig
	globalCfg *config.GlobalConfig
	tasks     *task.TaskService
	commands  CommandRunner
}

func New(logger *common.Logger, localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, tasks *task.TaskService, commands CommandRunner) *DrudgerService {
	return &DrudgerService{
		logger:    logger,
		localCfg:  localCfg,
		globalCfg: globalCfg,
		tasks:     tasks,
		commands:  commands,
	}
}

// RunTask hands one task to an agent. In dry run mode it only resolves and
// prints what the agent would be given, and writes nothing.
func (service *DrudgerService) RunTask(projectSlug string, requestedID task.TaskID, dryRun bool) error {
	taskToRun, err := service.tasks.GetTask(projectSlug, requestedID)
	if err != nil {
		return err
	}
	// The caller may have named the task by a prefix of its ID, which we should
	// not rely on in our internal logic.
	taskID := taskToRun.ID

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

	drudgerSlot, err := service.allocateDrudgerSlot(projectSlug)
	if err != nil {
		return err
	}
	drudgerName := formatDrudgerName(projectSlug, drudgerSlot, service.globalCfg.Drudger.Harness)

	workspace, err := common.WorkDir()
	if err != nil {
		return fmt.Errorf("could not work out where to run task %s: %w", taskID, err)
	}
	runDir := common.RunDir(workspace, string(taskID))

	plan, err := service.pickDrudgerCommand(projectSlug, drudgerSlot, workspace, runDir)
	if err != nil {
		return err
	}

	if dryRun {
		service.logger.Info("Drudger %d (%s) for task [%s] %s", drudgerSlot, drudgerName, taskToRun.ID, taskToRun.Title)
		service.logger.Info("Prompt (from %s):\n\n%s", promptSource, prompt)
		service.logger.Info("Commands:\n\n%s\n%s\n%s", formatArgv(plan.inspect), formatArgv(plan.create), formatArgv(plan.start))
		return nil
	}

	// TODO: before an agent is spawned, create a worktree for the task from the
	// default branch under the local worktrees dir, named wt-<task-id>, and
	// check out a branch named feat/<ticket-id>/<slug-from-task-title> in it.
	if err := service.ensureSandbox(plan, drudgerName, workspace); err != nil {
		return err
	}

	if err := writeRunPrompt(runDir, prompt); err != nil {
		return err
	}

	// TODO: add a command that pings a task's Session to tell whether it
	// is still alive, and frees the Drudger slot when it is not.
	if _, err := service.commands.Run(plan.start); err != nil {
		return fmt.Errorf("could not start Drudger %s for task %s: %w", drudgerName, taskID, err)
	}

	taskToRun.Status = task.StatusInProgress
	taskToRun.StartedAt = time.Now().UTC()
	taskToRun.DrudgerSlot = drudgerSlot
	taskToRun.SessionID = service.launchedSessionID(runDir)

	if err := service.tasks.UpdateTask(projectSlug, taskToRun); err != nil {
		return fmt.Errorf("Drudger %s is already working on task %s, but the task could not be marked as %q: %w", drudgerName, taskID, task.StatusInProgress, err)
	}

	service.logger.Info("Drudger %s is working on task [%s] %s", drudgerName, taskToRun.ID, taskToRun.Title)
	service.logger.Info("Run directory: %s", runDir)
	return nil
}

// launchedSessionID reads the session id the agent has written so far. An
// agent takes a moment to start up, so the stream is usually still empty at
// this point and an empty id is the normal answer. Reading the run directory
// later is what fills it in.
func (service *DrudgerService) launchedSessionID(runDir string) string {
	sessionID, err := readSessionID(runDir)
	if err != nil {
		service.logger.Error("%v, the task is recorded without a session id", err)
	}
	return sessionID
}

// ensureSandbox creates the Drudger's sandbox unless it already exists.
// Creating one that is already there fails, so the listing decides there.
// An existing sandbox is only reused when it holds the workspace of this run.
func (service *DrudgerService) ensureSandbox(plan sandboxPlan, drudgerName, workspace string) error {
	listing, err := service.commands.Run(plan.inspect)
	if err != nil {
		return fmt.Errorf("could not list the sandboxes to look for %s: %w", drudgerName, err)
	}

	existing, err := findSandbox(listing, drudgerName)
	if err != nil {
		return err
	}
	if existing != nil {
		return checkSandboxWorkspace(existing, workspace)
	}

	if _, err := service.commands.Run(plan.create); err != nil {
		return fmt.Errorf("could not create sandbox %s: %w", drudgerName, err)
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

// allocateDrudgerSlot picks the lowest free Drudger slot of a project's pool. A
// slot is taken for as long as the task holding it is in progress, so the pool
// is recomputed from the project's tasks on every run.
func (service *DrudgerService) allocateDrudgerSlot(projectSlug string) (int, error) {
	tasks, err := service.tasks.ListTasks(projectSlug)
	if err != nil {
		return 0, fmt.Errorf("could not read the project's tasks to work out which Drudgers are busy: %w", err)
	}

	occupied := occupiedDrudgerSlots(tasks)
	limit := config.ResolveMaxConcurrentDrudgers(service.localCfg, service.globalCfg)

	for candidate := 1; candidate <= limit; candidate++ {
		if !occupied[candidate] {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("all %d Drudgers of project %s are busy with %q tasks, wait for one to finish or raise %s in the config", limit, projectSlug, task.StatusInProgress, config.MaxConcurrentDrudgersKey)
}

// occupiedDrudgerSlots collects the Drudger slots held by in-progress tasks.
func occupiedDrudgerSlots(tasks []*task.Task) map[int]bool {
	occupied := make(map[int]bool)
	for _, candidate := range tasks {
		if candidate.Status == task.StatusInProgress && candidate.DrudgerSlot > 0 {
			occupied[candidate.DrudgerSlot] = true
		}
	}
	return occupied
}
