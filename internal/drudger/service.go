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
	"drudge/internal/git"
	"drudge/internal/task"
)

// TODO: Now dridger logis is couples with sbx quirks. At some point I need to
// move it inside an adapter. But for that we need another sandbox to compare
// with. And for that we need a reason for another sandbox.

// sbxDaemonRetryDelay is how long DRUDGE waits before giving the sbx daemon a
// second chance to come up.
const sbxDaemonRetryDelay = 2 * time.Second

// launchGracePeriod is how long a launch waits for the agent to write its
// first stream event. An agent writes that event a second or two after it
// starts, and a launch blocks for this whole period when none arrives.
const launchGracePeriod = 10 * time.Second

// launchPollInterval is how often a launch checks the stream while it waits
// out the grace period.
const launchPollInterval = 100 * time.Millisecond

// nukeCommand is what a user runs to kill a Drudger and the agent inside it.
// This package uses it only as an informative value, not a source of any
// decisions.
const nukeCommand = "drg drudger nuke"

// reclaimCommand is what a user runs to free the Drudger slots whose agent is
// gone.
const reclaimCommand = "drg drudger reclaim"

// initCommand is what a user runs to record the repositories of a project.
const initCommand = "drg project init"

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
	gitOps    git.Operations
	// daemonRetryDelay and launchGrace default to the constants above. A test
	// sets them to zero to skip the waits.
	daemonRetryDelay time.Duration
	launchGrace      time.Duration
	// resolvedLayout is what layout worked out, kept from the first call on.
	resolvedLayout *projectLayout
	// Some of the service methods live in other files of this package.
}

func New(logger *common.Logger, localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, tasks *task.TaskService, drudgers DrudgerRepository, commands CommandRunner, gitOps git.Operations) *DrudgerService {
	return &DrudgerService{
		logger:           logger,
		localCfg:         localCfg,
		globalCfg:        globalCfg,
		tasks:            tasks,
		drudgers:         drudgers,
		commands:         commands,
		gitOps:           gitOps,
		daemonRetryDelay: sbxDaemonRetryDelay,
		launchGrace:      launchGracePeriod,
	}
}

// ListDrudgers returns all Drudgers which currently exist in a Project.
func (service *DrudgerService) ListDrudgers(projectSlug string) ([]*Drudger, error) {
	layout, err := service.layout()
	if err != nil {
		return nil, err
	}

	drudgers, err := service.reclaimForListing(projectSlug, layout)
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
func (service *DrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	taskToRun, err := service.tasks.GetTask(projectSlug, requestedID)
	if err != nil {
		return err
	}

	layout, err := service.layout()
	if err != nil {
		return err
	}

	if isDryRun {
		if err := acceptRunnable(taskToRun); err != nil {
			return err
		}
		return service.describeRun(projectSlug, taskToRun, layout)
	}

	return service.launch(projectSlug, taskToRun.ID, layout, acceptRunnable)
}

// acceptRunnable refuses a task that is not waiting for an agent.
func acceptRunnable(taskToRun *task.Task) error {
	if taskToRun.Status != task.StatusTodo {
		return fmt.Errorf("task %s is %q, only %q tasks can be run", taskToRun.ID, taskToRun.Status, task.StatusTodo)
	}
	return nil
}

// RerunTask hands a task back to a Drudger and starts it over from scratch.
func (service *DrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	taskToRerun, err := service.tasks.GetTask(projectSlug, requestedID)
	if err != nil {
		return err
	}

	layout, err := service.layout()
	if err != nil {
		return err
	}

	var cameFrom task.TaskStatus
	accept := func(candidate *task.Task) error {
		cameFrom = candidate.Status
		return service.acceptRerunnable(projectSlug, layout, candidate)
	}

	if isDryRun {
		if err := accept(taskToRerun); err != nil {
			return err
		}
		return service.describeRun(projectSlug, taskToRerun, layout)
	}

	if err := service.launch(projectSlug, taskToRerun.ID, layout, accept); err != nil {
		return err
	}

	service.logger.Info("Task [%s] %s was %q, its previous run is cleared and it starts over", taskToRerun.ID, taskToRerun.Title, cameFrom)
	return nil
}

// acceptRerunnable refuses a task no agent has had yet, and one whose agent is
// still working.
func (service *DrudgerService) acceptRerunnable(projectSlug string, layout projectLayout, taskToRerun *task.Task) error {
	if !slices.Contains(rerunnableStatuses, taskToRerun.Status) {
		return fmt.Errorf(
			"task %s is %q, only %s tasks can be rerun",
			taskToRerun.ID, taskToRerun.Status, formatStatuses(rerunnableStatuses),
		)
	}

	working, err := service.workingDrudger(projectSlug, layout, taskToRerun.ID)
	if err != nil {
		return err
	}
	if working != nil {
		return fmt.Errorf(
			"Drudger %d (%s) is still working on task %s, wait for that Session to finish or run %s %d to kill it, then start the task over",
			working.Slot, working.Sandbox, taskToRerun.ID, nukeCommand, working.Slot,
		)
	}
	return nil
}

// RefuseWhileWorking refuses an edit or a removal of a task whose agent is
// still working, and names the Drudger running it. A task with no live Session
// goes through.
func (service *DrudgerService) RefuseWhileWorking(projectSlug string, taskToChange *task.Task) error {
	layout, err := service.layout()
	if err != nil {
		return err
	}

	working, err := service.workingDrudger(projectSlug, layout, taskToChange.ID)
	if err != nil {
		return err
	}
	if working == nil {
		return nil
	}
	return fmt.Errorf(
		"Drudger %d (%s) is still working on task %s, wait for that Session to finish or run %s %d to kill it, then try again",
		working.Slot, working.Sandbox, taskToChange.ID, nukeCommand, working.Slot,
	)
}

// RemoveRun deletes the run directory of a task and reports whether the task
// had one.
func (service *DrudgerService) RemoveRun(taskID task.TaskID) (bool, error) {
	layout, err := service.layout()
	if err != nil {
		return false, err
	}

	runDir := layout.RunDir(taskID)
	hasRunDir, err := common.Exists(runDir)
	if err != nil {
		return false, err
	}
	if !hasRunDir {
		return false, nil
	}

	if err := common.RemoveAll(runDir); err != nil {
		return false, err
	}
	return true, nil
}

// workingDrudger returns the Drudger whose agent is working on a task, and nil
// when none is.
//
// It checks the run directory and the Drudgers file. A dead agent leaves a run
// directory with no exit file, which on its own reads as a Session still
// working.
func (service *DrudgerService) workingDrudger(projectSlug string, layout projectLayout, taskID task.TaskID) (*Drudger, error) {
	isLive, err := service.sessionStillRunning(layout, taskID)
	if err != nil {
		return nil, err
	}
	if !isLive {
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
func (service *DrudgerService) sessionStillRunning(layout projectLayout, taskID task.TaskID) (bool, error) {
	report, err := readSessionReport(layout.RunDir(taskID), time.Now().UTC())
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

// launch starts an agent on a task and records the task as in progress. It
// holds the lock on the task for the whole launch, and accept re-checks the
// task against what is stored under that lock. A launch that finds the lock
// taken starts nothing.
//
// The lock covers the sandbox steps, which take minutes on a first launch.
// Only commands working on this same task wait that long, which is what a
// second launch of it should do.
func (service *DrudgerService) launch(projectSlug string, taskID task.TaskID, layout projectLayout, accept func(*task.Task) error) error {
	isStarted := false

	isStored, err := service.tasks.TryUpdateTask(projectSlug, taskID, func(taskToRun *task.Task) error {
		if err := accept(taskToRun); err != nil {
			return err
		}
		if err := service.startAgent(projectSlug, taskToRun, layout); err != nil {
			return err
		}
		isStarted = true
		return nil
	})
	if err != nil {
		if isStarted {
			return fmt.Errorf("an agent is already working on task %s, but the task could not be marked as %q: %w", taskID, task.StatusInProgress, err)
		}
		return err
	}
	if !isStored {
		return fmt.Errorf("another drudge command is working on task %s, wait for it to finish and run this again", taskID)
	}
	return nil
}

// startAgent claims a Drudger, starts an agent on a task and marks the task as
// in progress. The caller writes the task back.
func (service *DrudgerService) startAgent(projectSlug string, taskToRun *task.Task, layout projectLayout) error {
	taskID := taskToRun.ID

	prompt, _, err := service.renderTaskPrompt(taskToRun)
	if err != nil {
		return err
	}

	runDir := layout.RunDir(taskID)

	claimed, err := service.claimDrudger(projectSlug, taskID, layout)
	if err != nil {
		return err
	}

	isLaunched := false
	defer func() {
		// Manually release the drudger in case it failed to start.
		if !isLaunched {
			e := service.releaseDrudger(projectSlug, claimed.Slot, taskID)
			if e != nil {
				service.logger.Error("Drudger %d of project %s stays claimed for a run that never started: %v", claimed.Slot, projectSlug, e)
			}
		}
	}()

	space, err := service.resolveWorkspace(layout, claimed)
	if err != nil {
		return err
	}

	if err := service.ensureWorkspace(projectSlug, space); err != nil {
		return err
	}

	mounts := space.mounts(layout.RunsDir())

	plan, err := service.pickDrudgerCommand(claimed.Sandbox, space.Root, mounts, runDir)
	if err != nil {
		return err
	}

	if err := prepareRunDir(runDir, prompt); err != nil {
		return err
	}

	prepared, err := service.prepareWorkspace(space, taskToRun)
	if err != nil {
		return err
	}

	if err := service.ensureSandbox(projectSlug, claimed, plan, mounts); err != nil {
		return err
	}

	if err := service.commands.Start(plan.start); err != nil {
		return fmt.Errorf("could not start Drudger %s for task %s: %w", claimed.Sandbox, taskID, err)
	}

	if err := service.confirmLaunch(runDir, claimed.Sandbox, taskID); err != nil {
		return err
	}
	isLaunched = true

	taskToRun.StartRun(time.Now().UTC(), service.launchedSessionID(runDir))
	for _, repository := range prepared.Repositories {
		taskToRun.RecordStash(repository.Name, repository.Stash)
		taskToRun.RecordLanding(repository.Name, task.Landing{Branch: prepared.Branch, Base: repository.Base})
	}

	service.logger.Info("Drudger %s is working on task [%s] %s", claimed.Sandbox, taskToRun.ID, taskToRun.Title)
	service.logger.Info("Branch: %s", prepared.Branch)
	service.logger.Info("Run directory: %s", runDir)
	return nil
}

// renderTaskPrompt renders the prompt an agent is given for a task, and names
// where the template came from.
func (service *DrudgerService) renderTaskPrompt(taskToRun *task.Task) (prompt string, promptSource string, err error) {
	promptTemplate, promptSource, err := resolvePromptTemplate(service.localCfg, service.globalCfg)
	if err != nil {
		return "", "", err
	}

	prompt, err = renderPrompt(promptTemplate, taskToRun)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", promptSource, err)
	}
	return prompt, promptSource, nil
}

// describeRun prints the Drudger, the prompt and the commands a run would use,
// and writes nothing.
func (service *DrudgerService) describeRun(projectSlug string, taskToRun *task.Task, layout projectLayout) error {
	prompt, promptSource, err := service.renderTaskPrompt(taskToRun)
	if err != nil {
		return err
	}

	runDir := layout.RunDir(taskToRun.ID)

	wouldUse, err := service.previewDrudger(projectSlug, taskToRun.ID, layout)
	if err != nil {
		return err
	}

	space, err := service.resolveWorkspace(layout, wouldUse)
	if err != nil {
		return err
	}

	plan, err := service.pickDrudgerCommand(wouldUse.Sandbox, space.Root, space.mounts(layout.RunsDir()), runDir)
	if err != nil {
		return err
	}

	service.logger.Info("Drudger %d (%s) for task [%s] %s", wouldUse.Slot, wouldUse.Sandbox, taskToRun.ID, taskToRun.Title)
	service.logger.Info("Prompt (from %s):\n\n%s", promptSource, prompt)
	service.logger.Info("Commands:\n\n%s\n%s\n%s", formatArgv(plan.inspect.argv), formatArgv(plan.create.argv), formatArgv(plan.start))
	return nil
}

// confirmLaunch blocks until the agent writes its first stream event or the
// grace period runs out. Start returns as soon as the process is forked and
// drudge never learns its exit status, so an sbx exec that starts and exits is
// otherwise indistinguishable from a working agent.
func (service *DrudgerService) confirmLaunch(runDir string, sandboxName string, taskID task.TaskID) error {
	deadline := time.Now().Add(service.launchGrace)

	for {
		hasStarted, err := agentStarted(runDir)
		if err != nil {
			return err
		}
		if hasStarted {
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
// line carrying the id can still be half written, so an empty id is a normal
// answer.
func (service *DrudgerService) launchedSessionID(runDir string) string {
	sessionID, err := readSessionID(runDir)
	if err != nil {
		service.logger.Error("%v, the task is recorded without a session id", err)
	}
	return sessionID
}

// ensureSandbox creates the Drudger's sandbox unless it already exists.
// Creating one that is already there fails, so the listing decides there.
// An existing sandbox is only reused when it holds every mount of this run.
//
// What the listing says about the sandbox is recorded as the Drudger's
// sandbox health.
func (service *DrudgerService) ensureSandbox(projectSlug string, claimed *Drudger, plan sandboxPlan, mounts []string) error {
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

	if err := checkSandboxWorkspace(existing, mounts, claimed.Slot); err != nil {
		service.recordSandboxHealth(projectSlug, claimed.Slot, SandboxMisplaced)
		return err
	}
	service.recordSandboxHealth(projectSlug, claimed.Slot, SandboxUsable)
	return nil
}

// observeSandboxes returns which sandboxes of the configured environment are
// running. A listing that fails is an error here, because a partial answer
// would free a slot on a guess.
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
func (service *DrudgerService) listSandboxes(inspect sandboxCommand) (string, error) {
	listing, stderr, err := service.runSbx(inspect)

	// A call killed for outrunning its timeout is not the daemon-not-up
	// condition, and retrying it would wait out a second full timeout.
	if err != nil && daemonWouldNotStart(stderr) && !timedOut(err) {
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

// runSbx runs one sbx command and logs when the call had to start the sbx
// daemon, which explains the delay the user sees. A call killed for outrunning
// its timeout is reported as a daemon that stopped answering.
func (service *DrudgerService) runSbx(command sandboxCommand) (string, string, error) {
	stdout, stderr, err := service.commands.Run(command.argv, command.timeout)
	if daemonJustStarted(stderr) {
		service.logger.Info("The sbx daemon was not running, sbx has just started it")
	}
	if timedOut(err) {
		return stdout, stderr, fmt.Errorf("the sbx daemon stopped answering, run %s to see what is wrong with it: %w", sbxDaemonStatusCommand, err)
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
