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

// launchGracePeriod is how long a launch waits for the agent to write its
// first stream event. The agent writes it in the first second or two, so an
// empty stream after this means the process is not there. A launch blocks for
// this long at worst, so it stays short.
const launchGracePeriod = 10 * time.Second

// launchPollInterval is how often a launch checks the stream while it waits
// out the grace period.
const launchPollInterval = 100 * time.Millisecond

// nukeCommand is what a user runs to kill a Drudger and the agent inside it.
// This package uses it only as an informative value, not a source of any
// decisions.
const nukeCommand = "drg drudger nuke"

// reclaimCommand is what a user runs to free the Drudger slots whose agent is
// gone. This package uses it only as an informative value, not a source of any
// decisions.
const reclaimCommand = "drg drudger reclaim"

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
	// launchGrace is how long a launch waits for the agent to write its first
	// event, held here so a test does not have to sit through it.
	launchGrace time.Duration
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
		launchGrace:      launchGracePeriod,
	}
}

// ListDrudgers returns all Drudgers which currently exist in a Project.
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

	working, err := service.workingDrudger(projectSlug, workspace, taskToRerun.ID)
	if err != nil {
		return err
	}
	if working != nil {
		return fmt.Errorf(
			"Drudger %d (%s) is still working on task %s, wait for that Session to finish or run %s %d to kill it, then start the task over",
			working.Slot, working.Sandbox, taskToRerun.ID, nukeCommand, working.Slot,
		)
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

// workingDrudger returns the Drudger whose agent is working on a task, and
// nil when none is.
//
// Both facts have to hold. The run directory has to have no exit file, and a
// Drudger has to still hold the task. The Drudgers file is what says whether
// an agent exists, so a run directory a dead agent left unfinished stops
// counting once ReclaimDrudgers has cleared its slot.
func (service *DrudgerService) workingDrudger(projectSlug string, workspace string, taskID task.TaskID) (*Drudger, error) {
	live, err := service.sessionStillRunning(workspace, taskID)
	if err != nil {
		return nil, err
	}
	if !live {
		return nil, nil
	}

	drudgers, err := service.drudgers.ListDrudgers(projectSlug)
	if err != nil {
		return nil, fmt.Errorf("an agent may still be working on task %s, but the Drudgers of project %s could not be read to tell: %w", taskID, projectSlug, err)
	}
	return drudgerHoldingTask(drudgers, taskID), nil
}

// sessionStillRunning reports whether the run directory of a task says its
// Session has not ended.
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

	// The run directory is made before the sandbox steps, which take minutes
	// on a first launch. That keeps the gap between claiming a slot and
	// creating its run directory short, so ReclaimDrudgers can read a missing
	// run directory as a launch that never happened.
	if err := prepareRunDir(runDir, prompt); err != nil {
		return err
	}

	// TODO: before an agent is spawned, create a worktree for the task from the
	// default branch under the local worktrees dir, named wt-<task-id>, and
	// check out a branch named feat/<ticket-id>/<slug-from-task-title> in it.
	if err := service.ensureSandbox(projectSlug, drudger, plan, workspace); err != nil {
		return err
	}

	if err := service.commands.Start(plan.start); err != nil {
		return fmt.Errorf("could not start Drudger %s for task %s: %w", drudger.Sandbox, taskID, err)
	}

	if err := service.confirmLaunch(runDir, drudger.Sandbox, taskID); err != nil {
		return err
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

// confirmLaunch blocks until the agent writes its first stream event or the
// grace period runs out. Start returns as soon as the process is forked and
// drudge never learns its exit status, so an sbx exec that dies a moment later
// is otherwise indistinguishable from a working agent. Failing here leaves the
// task in todo and releases the claimed slot.
func (service *DrudgerService) confirmLaunch(runDir string, sandboxName string, taskID task.TaskID) error {
	deadline := time.Now().Add(service.launchGrace)

	for {
		started, err := agentStarted(runDir)
		if err != nil {
			return err
		}
		if started {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf(
				"Drudger %s produced no output within %s of being started on task %s, so its agent did not start, check that the sandbox works and look in %s",
				sandboxName, service.launchGrace, taskID, runDir,
			)
		}
		time.Sleep(launchPollInterval)
	}
}

// launchedSessionID reads the session id the agent has written so far. The
// stream has content by this point, but the line holding the id can be
// partially written, so an empty id is a normal answer. Later reads of the run
// directory fill it in.
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
	listing, err := service.listSandboxes(plan.inspect)
	if err != nil {
		return fmt.Errorf("%w, so DRUDGE cannot tell whether sandbox %s is there", err, claimed.Sandbox)
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

// observeSandboxes returns which sandboxes of the configured environment are
// running. The result decides whether a slot is freed, so a listing that fails
// stops the caller with an error.
func (service *DrudgerService) observeSandboxes(projectSlug string) (map[string]bool, error) {
	inspect, err := service.pickInspectCommand()
	if err != nil {
		return nil, err
	}

	listing, err := service.listSandboxes(inspect)
	if err != nil {
		return nil, fmt.Errorf("%w, so DRUDGE cannot tell which Drudgers of project %s still have an agent in them", err, projectSlug)
	}

	running, err := readRunningSandboxes(listing)
	if err != nil {
		return nil, fmt.Errorf("%w, so DRUDGE cannot tell which Drudgers of project %s still have an agent in them", err, projectSlug)
	}
	return running, nil
}

// listSandboxes lists the sandboxes, coping with an sbx daemon that is not up
// yet.
// TODO: This method is OK for now while I'm still trying to make everything
// work. But when I inevitably do, I need to move out all the sbx quirks into
// an adapter, because service and sbx are now a bit too close to each other.
func (service *DrudgerService) listSandboxes(inspect []string) (string, error) {
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
		return "", fmt.Errorf("could not list the sandboxes: %w", err)
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
