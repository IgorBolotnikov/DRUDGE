// Package drudger hands tasks to agents working in sandboxes
package drudger

import (
	"cmp"
	"fmt"
	"slices"
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
	drudgers  DrudgerRepository
	commands  CommandRunner
	// Some of the service methods also live in allocation.go and nuke.go
}

func New(logger *common.Logger, localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, tasks *task.TaskService, drudgers DrudgerRepository, commands CommandRunner) *DrudgerService {
	return &DrudgerService{
		logger:    logger,
		localCfg:  localCfg,
		globalCfg: globalCfg,
		tasks:     tasks,
		drudgers:  drudgers,
		commands:  commands,
	}
}

// ListDrudgers returns all Drudgers which currently exist in a Project.
func (service *DrudgerService) ListDrudgers(projectSlug string) ([]*Drudger, error) {
	drudgers, err := service.drudgers.ListDrudgers(projectSlug)
	if err != nil {
		return nil, fmt.Errorf("could not list the Drudgers of project %s: %w", projectSlug, err)
	}

	slices.SortFunc(drudgers, func(first, second *Drudger) int {
		return cmp.Compare(first.Slot, second.Slot)
	})
	return drudgers, nil
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

	workspace, err := common.WorkDir()
	if err != nil {
		return fmt.Errorf("could not work out where to run task %s: %w", taskID, err)
	}
	runDir := common.RunDir(workspace, string(taskID))

	if dryRun {
		return service.describeRun(projectSlug, taskToRun, workspace, runDir, prompt, promptSource)
	}

	drudger, err := service.claimDrudger(projectSlug, taskID, workspace)
	if err != nil {
		return err
	}

	launched := false
	defer func() {
		// Manually release the drudger in case it failed to start.
		if !launched {
			e := service.releaseDrudger(projectSlug, drudger.Slot, taskID)
			if e != nil {
				service.logger.Error("Drudger %d of project %s stays claimed for a run that never started: %v", drudger.Slot, projectSlug, e)
			}
		}
	}()

	plan, err := service.pickDrudgerCommand(drudger.Sandbox, workspace, runDir)
	if err != nil {
		return err
	}

	// TODO: before an agent is spawned, create a worktree for the task from the
	// default branch under the local worktrees dir, named wt-<task-id>, and
	// check out a branch named feat/<ticket-id>/<slug-from-task-title> in it.
	if err := service.ensureSandbox(projectSlug, drudger, plan, workspace); err != nil {
		return err
	}

	if err := writeRunPrompt(runDir, prompt); err != nil {
		return err
	}

	// TODO: add a command that pings a task's Session to tell whether it
	// is still alive, and frees the Drudger slot when it is not.
	if _, err := service.commands.Run(plan.start); err != nil {
		return fmt.Errorf("could not start Drudger %s for task %s: %w", drudger.Sandbox, taskID, err)
	}
	launched = true

	taskToRun.Status = task.StatusInProgress
	taskToRun.StartedAt = time.Now().UTC()
	taskToRun.SessionID = service.launchedSessionID(runDir)

	if err := service.tasks.UpdateTask(projectSlug, taskToRun); err != nil {
		return fmt.Errorf("Drudger %s is already working on task %s, but the task could not be marked as %q: %w", drudger.Sandbox, taskID, task.StatusInProgress, err)
	}

	service.logger.Info("Drudger %s is working on task [%s] %s", drudger.Sandbox, taskToRun.ID, taskToRun.Title)
	service.logger.Info("Run directory: %s", runDir)
	return nil
}

func (service *DrudgerService) describeRun(projectSlug string, taskToRun *task.Task, workspace, runDir, prompt, promptSource string) error {
	wouldUse, err := service.previewDrudger(projectSlug, taskToRun.ID, workspace)
	if err != nil {
		return err
	}

	plan, err := service.pickDrudgerCommand(wouldUse.Sandbox, workspace, runDir)
	if err != nil {
		return err
	}

	service.logger.Info("Drudger %d (%s) for task [%s] %s", wouldUse.Slot, wouldUse.Sandbox, taskToRun.ID, taskToRun.Title)
	service.logger.Info("Prompt (from %s):\n\n%s", promptSource, prompt)
	service.logger.Info("Commands:\n\n%s\n%s\n%s", formatArgv(plan.inspect), formatArgv(plan.create), formatArgv(plan.start))
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
//
// What the listing says about the sandbox is recorded as the Drudger's health.
func (service *DrudgerService) ensureSandbox(projectSlug string, claimed *Drudger, plan sandboxPlan, workspace string) error {
	listing, err := service.commands.Run(plan.inspect)
	if err != nil {
		return fmt.Errorf("could not list the sandboxes to look for %s: %w", claimed.Sandbox, err)
	}

	existing, err := findSandbox(listing, claimed.Sandbox)
	if err != nil {
		return err
	}

	if existing == nil {
		if _, err := service.commands.Run(plan.create); err != nil {
			service.recordHealth(projectSlug, claimed.Slot, HealthGone)
			return fmt.Errorf("could not create sandbox %s: %w", claimed.Sandbox, err)
		}
		service.recordHealth(projectSlug, claimed.Slot, HealthUsable)
		return nil
	}

	if err := checkSandboxWorkspace(existing, workspace); err != nil {
		service.recordHealth(projectSlug, claimed.Slot, HealthMisplaced)
		return err
	}
	service.recordHealth(projectSlug, claimed.Slot, HealthUsable)
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
