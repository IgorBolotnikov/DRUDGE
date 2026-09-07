// Package drudger hands tasks to agents working in sandboxes
package drudger

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

// TODO: Now dridger logis is couples with sbx quirks. At some point I need to
// move it inside an adapter. But for that we need another sandbox to compare
// with. And for that we need a reason for another sandbox.

// sbxDaemonRetryDelay is how long DRUDGE waits before giving the sbx daemon a
// second chance to come up.
const sbxDaemonRetryDelay = 2 * time.Second

// nukeCommand is what a user runs to kill a Drudger and the agent inside it.
// This package uses it only as an informative value, not a source of any
// decisions.
const nukeCommand = "drg drudger nuke"

// rerunnableStatuses are the task statuses a rerun accepts. They are the ones
// an agent has actually had.
var rerunnableStatuses = []task.TaskStatus{task.StatusInProgress, task.StatusFuckedUp}

type DrudgerService struct {
	logger    *common.Logger
	localCfg  *config.LocalConfig
	globalCfg *config.GlobalConfig
	tasks     *task.TaskService
	drudgers  DrudgerRepository
	commands  CommandRunner
	// daemonRetryDelay is the wait before a retried sbx call, held here so a
	// test does not have to sit through it.
	daemonRetryDelay time.Duration
	// Some of the service methods also live in allocation.go and nuke.go
}

func New(logger *common.Logger, localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, tasks *task.TaskService, drudgers DrudgerRepository, commands CommandRunner) *DrudgerService {
	return &DrudgerService{
		logger:           logger,
		localCfg:         localCfg,
		globalCfg:        globalCfg,
		tasks:            tasks,
		drudgers:         drudgers,
		commands:         commands,
		daemonRetryDelay: sbxDaemonRetryDelay,
	}
}

// ListDrudgers returns all Drudgers which currently exist in a Project.
//
// It looks at the run directories before it reports, so a Drudger whose
// Session has ended reads as idle. A Drudger record stays a snapshot of what
// drudge last observed, and LastChecked still says how old that is.
func (service *DrudgerService) ListDrudgers(projectSlug string) ([]*Drudger, error) {
	workspace, err := common.WorkDir()
	if err != nil {
		return nil, fmt.Errorf("could not work out where the Drudgers of project %s run: %w", projectSlug, err)
	}

	drudgers, err := service.reclaimForListing(projectSlug, workspace)
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

	if taskToRun.Status != task.StatusTodo {
		return fmt.Errorf("task %s is %q, only %q tasks can be run", taskToRun.ID, taskToRun.Status, task.StatusTodo)
	}

	workspace, err := common.WorkDir()
	if err != nil {
		return fmt.Errorf("could not work out where to run task %s: %w", taskToRun.ID, err)
	}

	return service.launch(projectSlug, taskToRun, workspace, dryRun)
}

// RerunTask hands a task back to a Drudger and starts it over from scratch.
func (service *DrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, dryRun bool) error {
	taskToRerun, err := service.tasks.GetTask(projectSlug, requestedID)
	if err != nil {
		return err
	}

	if !slices.Contains(rerunnableStatuses, taskToRerun.Status) {
		return fmt.Errorf(
			"task %s is %q, only %s tasks can be rerun",
			taskToRerun.ID, taskToRerun.Status, formatStatuses(rerunnableStatuses),
		)
	}

	workspace, err := common.WorkDir()
	if err != nil {
		return fmt.Errorf("could not work out where to run task %s: %w", taskToRerun.ID, err)
	}

	live, err := service.sessionStillRunning(workspace, taskToRerun.ID)
	if err != nil {
		return err
	}
	if live {
		return service.refuseLiveRerun(projectSlug, taskToRerun)
	}

	cameFrom := taskToRerun.Status
	if err := service.launch(projectSlug, taskToRerun, workspace, dryRun); err != nil {
		return err
	}
	if !dryRun {
		service.logger.Info("Task [%s] %s was %q, its previous run is cleared and it starts over", taskToRerun.ID, taskToRerun.Title, cameFrom)
	}
	return nil
}

// sessionStillRunning reports whether an agent is currently working on a task.
func (service *DrudgerService) sessionStillRunning(workspace string, taskID task.TaskID) (bool, error) {
	report, err := readSessionReport(common.RunDir(workspace, string(taskID)), time.Now().UTC())
	if errors.Is(err, errNoRunDirectory) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !report.Finished(), nil
}

// refuseLiveRerun explains why a task with a working agent cannot start over.
func (service *DrudgerService) refuseLiveRerun(projectSlug string, taskToRerun *task.Task) error {
	drudgers, err := service.drudgers.ListDrudgers(projectSlug)
	if err != nil {
		return fmt.Errorf("an agent is still working on task %s, but the Drudgers of project %s could not be read to name it: %w", taskToRerun.ID, projectSlug, err)
	}

	holder := drudgerHoldingTask(drudgers, taskToRerun.ID)
	if holder == nil {
		return fmt.Errorf("an agent is still working on task %s, wait for that Session to finish before starting the task over", taskToRerun.ID)
	}
	return fmt.Errorf(
		"Drudger %d (%s) is still working on task %s, wait for that Session to finish or run %s %d to kill it, then start the task over",
		holder.Slot, holder.Sandbox, taskToRerun.ID, nukeCommand, holder.Slot,
	)
}

// formatStatuses lists task statuses for an error message.
func formatStatuses(statuses []task.TaskStatus) string {
	quoted := make([]string, 0, len(statuses))
	for _, status := range statuses {
		quoted = append(quoted, fmt.Sprintf("%q", status))
	}
	return strings.Join(quoted, " and ")
}

// launch resolves the prompt, claims a Drudger, makes sure its sandbox is
// there, clears the run directory and starts the agent.
func (service *DrudgerService) launch(projectSlug string, taskToRun *task.Task, workspace string, dryRun bool) error {
	taskID := taskToRun.ID

	promptTemplate, promptSource, err := resolvePromptTemplate(service.localCfg, service.globalCfg)
	if err != nil {
		return err
	}

	prompt, err := renderPrompt(promptTemplate, taskToRun)
	if err != nil {
		return fmt.Errorf("%s: %w", promptSource, err)
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

	if err := prepareRunDir(runDir, prompt); err != nil {
		return err
	}

	// TODO: add a command that pings a task's Session to tell whether it
	// is still alive, and frees the Drudger slot when it is not.
	if err := service.commands.Start(plan.start); err != nil {
		return fmt.Errorf("could not start Drudger %s for task %s: %w", drudger.Sandbox, taskID, err)
	}
	launched = true

	taskToRun.StartRun(time.Now().UTC(), service.launchedSessionID(runDir))

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
// What the listing says about the sandbox is recorded as the Drudger's
// sandbox health.
func (service *DrudgerService) ensureSandbox(projectSlug string, claimed *Drudger, plan sandboxPlan, workspace string) error {
	listing, err := service.listSandboxes(plan.inspect, claimed.Sandbox)
	if err != nil {
		return err
	}

	existing, err := findSandbox(listing, claimed.Sandbox)
	if err != nil {
		return err
	}

	if existing == nil {
		if _, _, err := service.runSbx(plan.create); err != nil {
			service.recordSandboxHealth(projectSlug, claimed.Slot, SandboxGone)
			return fmt.Errorf("could not create sandbox %s: %w", claimed.Sandbox, err)
		}
		service.recordSandboxHealth(projectSlug, claimed.Slot, SandboxUsable)
		return nil
	}

	if err := checkSandboxWorkspace(existing, workspace); err != nil {
		service.recordSandboxHealth(projectSlug, claimed.Slot, SandboxMisplaced)
		return err
	}
	service.recordSandboxHealth(projectSlug, claimed.Slot, SandboxUsable)
	return nil
}

// listSandboxes lists the sandboxes, coping with an sbx daemon that is not up
// yet.
// TODO: This method is OK for now while I'm still trying to make everything
// work. But when I inevitably do, I need to move out all the sbx quirks into
// an adapter, because service and sbx are now a bit too close to each other.
func (service *DrudgerService) listSandboxes(inspect []string, sandboxName string) (string, error) {
	listing, stderr, err := service.runSbx(inspect)

	if err != nil && daemonWouldNotStart(stderr) {
		service.logger.Info("The sbx daemon did not come up, DRUDGE gives it one more try")
		time.Sleep(service.daemonRetryDelay)

		listing, stderr, err = service.runSbx(inspect)
		if err != nil && daemonWouldNotStart(stderr) {
			return "", fmt.Errorf("the sbx daemon would not start, run %s to see what is wrong with it: %w", sbxDaemonStatusCommand, err)
		}
	}

	if err != nil {
		return "", fmt.Errorf("could not list the sandboxes to look for %s: %w", sandboxName, err)
	}
	return listing, nil
}

// runSbx runs one sbx command and says when the call had to bring the sandbox
// daemon up to explain the relay in running the command.
func (service *DrudgerService) runSbx(argv []string) (string, string, error) {
	stdout, stderr, err := service.commands.Run(argv)
	if daemonJustStarted(stderr) {
		service.logger.Info("The sbx daemon was not running, sbx has just started it")
	}
	return stdout, stderr, err
}

// prepareRunDir clears whatever the previous run left in the run directory and
// puts the rendered prompt there, where the agent reads it from inside its
// sandbox.
func prepareRunDir(runDir string, prompt string) error {
	if err := common.RemoveAll(runDir); err != nil {
		return err
	}
	if err := common.EnsureDir(runDir); err != nil {
		return err
	}
	return common.WriteFile(common.RunPromptPath(runDir), prompt)
}
