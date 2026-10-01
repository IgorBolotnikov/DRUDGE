package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const (
	unblockedLabel = "Unblocked"
	adviceLabel    = "Advice"
)

// cliProgress renders the events a domain service reports as the lines drg
// prints.
type cliProgress struct {
	log *common.Logger
	out *printer
}

// newCLIProgress builds a progress port that prints through out.
func newCLIProgress(out *printer) *cliProgress {
	return &cliProgress{log: out.log, out: out}
}

func (p *cliProgress) Report(event any) {
	switch event := event.(type) {
	case task.TaskCreated:
		p.out.done("Created task %s", p.out.task(event.Task))
	case task.TaskRemovalDeclined:
		p.log.Info("Left task [%s] %s alone", event.Task.ID, event.Task.Title)
	case task.TaskRemoved:
		p.log.Info("Removed task [%s] %s", event.Task.ID, event.Task.Title)
		if event.HasRun {
			p.log.Info("Its run directory went with it")
		}
	case task.TasksUnblocked:
		p.log.Info("Took it off the blockers of %s", task.FormatTaskCount(event.Count))
	case task.TasksUngrouped:
		p.log.Info("Ungrouped %s that belonged to it", task.FormatTaskCount(event.Count))
	case task.BranchesCleanupFailed:
		p.out.warn("Task %s is removed, but the branches it left could not be cleaned up: %v", event.TaskID, event.Err)
	case task.TaskUnlinkFailed:
		p.warnTaskUnlinkFailed(event)
	case task.TaskMarkedDone:
		p.out.done("Task %s is %q", p.out.task(event.Task), event.Task.Status)
	case task.TaskEdited:
		p.out.done("Updated task %s, it is now %q", p.out.task(event.Task), event.Task.Status)
	case project.ProjectCreated:
		// Project init prints the result line itself, after the warnings
		// about its repositories.
	case project.ProjectRenamed:
		p.out.done("Renamed project %s from %q to %q", event.Slug, event.OldName, event.NewName)
	case project.ProjectRemoved:
		p.out.done("Removed project %s", event.Name)
	case project.ProjectAlreadyGone:
		p.out.skip("Project %s was already gone", event.Slug)
	case release.DownloadStarted:
		p.out.step("Downloading %s (%s)", event.ArchiveName, event.Version)
	default:
		p.reportDrudger(event)
	}
}

func (p *cliProgress) warnTaskUnlinkFailed(event task.TaskUnlinkFailed) {
	if event.IsHeld {
		p.out.warn("Task %s is removed, but another drudge command is working on task %s, which still names it", event.RemovedID, event.LinkedID)
		return
	}
	p.out.warn("Task %s is removed, but task %s still names it: %v", event.RemovedID, event.LinkedID, event.Err)
}

// reportDrudger renders the events the drudger service reports.
func (p *cliProgress) reportDrudger(event any) {
	switch event := event.(type) {
	case drudger.DrudgerClaimed:
		p.out.header("Task %s → Drudger %d (%s)", p.out.task(event.Task), event.Slot, event.Sandbox)
	case drudger.BaseFetchStarted:
		p.out.step("Fetching %s of %s from %s", event.Branch, event.Repository, event.Remote)
	case drudger.BaseFetchFailed:
		p.out.warn("Could not fetch %s of %s: %v", event.Branch, event.Repository, event.Err)
	case drudger.StaleBaseUsed:
		p.out.warn("Work on %s is cut from %s", event.Repository, describeBase(event.Ref, event.Commit))
	case drudger.WorktreeCreationStarted:
		p.out.step("Creating the workspace of %s at %s", event.Repository, event.Path)
	case drudger.WorktreeStashed:
		p.out.done("The workspace of %s held uncommitted changes, they are stashed at %s", event.Repository, git.ShortSHA(event.Commit))
	case drudger.BranchCheckoutStarted:
		p.out.step("Putting the workspace on branch %s", event.Branch)
	case drudger.SandboxLookupStarted:
		p.out.step("Looking for sandbox %s", event.Sandbox)
	case drudger.SandboxCreationStarted:
		p.out.step("Creating sandbox %s, the first one pulls its image, up to %s", event.Sandbox, event.Timeout)
	case drudger.SandboxReused:
		p.out.done("Sandbox %s exists, reusing it", event.Sandbox)
	case drudger.SbxOutput:
		p.out.detail("%s: %s", event.Binary, event.Line)
	case drudger.SbxDaemonRetried:
		p.out.warn("The sbx daemon did not come up, DRUDGE gives it one more try")
	case drudger.SbxDaemonStarted:
		p.out.done("The sbx daemon was not running, sbx started it")
	case drudger.AgentLaunchStarted:
		p.out.step("Starting the agent, waiting up to %s for its first output", event.GracePeriod)
	case drudger.AgentLaunched:
		p.out.result("Drudger %s is working on task %s", event.Sandbox, p.out.task(event.Task))
		p.out.field("Branch", event.Branch)
		p.out.field("Run dir", event.RunDir)
		p.out.flush()
	case drudger.TaskRestarted:
		p.out.done("Task %s was %q, it starts over", p.out.task(event.Task), event.CameFrom)
	case drudger.RunDescribed:
		commands := make([]string, 0, len(event.Commands))
		for _, argv := range event.Commands {
			commands = append(commands, formatArgv(argv))
		}
		p.out.header("Dry run of task %s on Drudger %d (%s)", p.out.task(event.Task), event.Slot, event.Sandbox)
		p.out.field("Prompt from", event.PromptSource)
		p.out.block(event.Prompt)
		p.out.field("Commands", "")
		p.out.block(strings.Join(commands, "\n"))
		p.out.skipResult("Nothing ran, it was a dry run")
	case drudger.DrudgersAboveLimit:
		names := make([]string, 0, len(event.Drudgers))
		for _, above := range event.Drudgers {
			names = append(names, fmt.Sprintf("slot %d (%s)", above.Slot, above.Sandbox))
		}
		p.out.warn(
			"Project %s has Drudgers above the %s limit of %d, they get no tasks: %s. Raise %s to put them back to work, or nuke them if you are done with them",
			event.ProjectSlug, drudger.MaxConcurrentDrudgersKey, event.Limit, strings.Join(names, ", "), drudger.MaxConcurrentDrudgersKey,
		)
	case drudger.DrudgerListBehind:
		p.out.warn("Another drudge command holds the Drudgers of project %s, so this list is what was last written and may be behind", event.ProjectSlug)
	case drudger.SessionRecordingStarted:
		p.out.header("Recording the Session of task %s", p.out.task(event.Task))
	case drudger.WorkFoundOnBranch:
		p.out.done("The agent left %s on branch %s, its work is there", event.Repository, event.Branch)
	case drudger.RescueBranchCreated:
		p.out.done("The agent left %s on no branch, its commits are on %s", event.Repository, event.Branch)
	case drudger.EmptyBranchDropped:
		p.out.skip("The agent committed nothing in %s, branch %s is deleted", event.Repository, event.Branch)
	case drudger.SessionRecorded:
		p.sessionResult(event.Status)("Task %s %s, it is %s", p.out.task(event.Task), event.Status, event.Task.Status)
	case drudger.DependentsUnblocked:
		label := unblockedLabel
		for _, dependent := range event.Tasks {
			p.out.field(label, p.out.task(dependent))
			label = ""
		}
		p.out.flush()
	case drudger.SessionLeftUnrecorded:
		p.out.skip("Another drudge command is working on task %s, so this check reports the run directory without recording it", event.TaskID)
	case drudger.RunRefused:
		p.out.resultWarn("The vendor refused task %s (%s), it is back in %s", p.out.task(event.Task), event.Task.VendorErrorClass, event.Task.Status)
		p.out.field(adviceLabel, vendorErrorAdvice(event.Task.VendorErrorClass))
		p.out.flush()
	case drudger.DrudgerNuked:
		p.log.Info("Drudger %d is gone, sandbox %s was deleted", event.Slot, event.Sandbox)
	case drudger.SandboxAlreadyGone:
		p.log.Info("Sandbox %s was already gone", event.Sandbox)
	case drudger.TaskKilled:
		p.log.Info("Task [%s] %s is %s, its agent was killed with the Drudger", event.Task.ID, event.Task.Title, event.Task.Status)
	case drudger.BranchOfUnknownRepositoryKept:
		p.log.Info("Branch %s stays, project %s records no repository %s", event.Branch, event.ProjectSlug, event.Repository)
	case drudger.BranchWithCommitsKept:
		p.log.Info("Branch %s of repository %s holds commits, it stays", event.Branch, event.Repository)
	case drudger.EmptyBranchRemoved:
		p.log.Info("Branch %s of repository %s held nothing, it is deleted", event.Branch, event.Repository)
	case drudger.HealthRecordFailed:
		p.warnHealthRecordFailed(event)
	case drudger.IdleWorkspaceParkFailed:
		p.warnIdleWorkspaceParkFailed(event)
	case drudger.RunCloseOutFailed:
		p.warnRunCloseOutFailed(event)
	case drudger.WorkspaceNukeFailed:
		p.warnWorkspaceNukeFailed(event)
	case drudger.BranchCleanupFailed:
		p.warnBranchCleanupFailed(event)
	case drudger.UnblockedLookupFailed:
		p.out.warn("Task %s is done, but the tasks it unblocked could not be worked out: %v", event.TaskID, event.Err)
	case drudger.DrudgerReleaseFailed:
		p.out.warn("Drudger %d of project %s stays claimed for a run that never started: %v", event.Slot, event.ProjectSlug, event.Err)
	case drudger.SessionIDReadFailed:
		p.out.warn("%v, the task is recorded without a session id", event.Err)
	}
}

// sessionResult picks the result line of a recorded Session by the role its
// status prints in.
func (p *cliProgress) sessionResult(status drudger.SessionStatus) func(format string, args ...any) {
	switch sessionStatusRoles[status] {
	case theme.RoleWarning:
		return p.out.resultWarn
	case theme.RoleError:
		return p.out.resultFailed
	default:
		return p.out.result
	}
}

func (p *cliProgress) warnHealthRecordFailed(event drudger.HealthRecordFailed) {
	switch event.Part {
	case drudger.SandboxPart:
		p.out.warn("The sandbox of Drudger %d of project %s is %s, but that could not be recorded: %v", event.Slot, event.ProjectSlug, event.Health, event.Err)
	case drudger.WorkspacePart:
		p.out.warn("The workspace of Drudger %d of project %s is %s, but that could not be recorded: %v", event.Slot, event.ProjectSlug, event.Health, event.Err)
	case drudger.AgentPart:
		p.out.warn("The agent that ran task %s of project %s is %s, but that could not be recorded: %v", event.TaskID, event.ProjectSlug, event.Health, event.Err)
	}
}

func (p *cliProgress) warnIdleWorkspaceParkFailed(event drudger.IdleWorkspaceParkFailed) {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		p.out.warn("Drudger %d of project %s holds no task, but the workspace it works in could not be read: %v", event.Slot, event.ProjectSlug, event.Err)
	case drudger.WorkspaceParkStep:
		p.out.warn("Drudger %d of project %s holds no task, but its workspace could not be parked: %v", event.Slot, event.ProjectSlug, event.Err)
	}
}

func (p *cliProgress) warnRunCloseOutFailed(event drudger.RunCloseOutFailed) {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		p.out.warn("The Session of task %s is over, but the workspace it ran in could not be read: %v", event.TaskID, event.Err)
	case drudger.WorkspaceParkStep:
		p.out.warn("The Session of task %s is over, but the workspace it ran in could not be parked: %v", event.TaskID, event.Err)
	case drudger.RepositoryCloseOutStep:
		p.out.warn("The Session of task %s is over, but where its work in repository %s is could not be worked out: %v", event.TaskID, event.Repository, event.Err)
	}
}

func (p *cliProgress) warnWorkspaceNukeFailed(event drudger.WorkspaceNukeFailed) {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		p.out.warn("Drudger %d of project %s is being nuked, but the workspace it works in could not be read: %v", event.Slot, event.ProjectSlug, event.Err)
	case drudger.WorktreeRemovalStep:
		p.out.warn("Drudger %d of project %s is being nuked, but its worktree of repository %s could not be taken out: %v", event.Slot, event.ProjectSlug, event.Repository, event.Err)
	}
}

func (p *cliProgress) warnBranchCleanupFailed(event drudger.BranchCleanupFailed) {
	switch event.Step {
	case drudger.RepositoryReadStep:
		p.out.warn("Could not read repository %s, branch %s stays: %v", event.Repository, event.Branch, event.Err)
	case drudger.BranchReadStep:
		p.out.warn("Could not read branch %s of repository %s: %v", event.Branch, event.Repository, event.Err)
	case drudger.BranchInspectStep:
		p.out.warn("Could not read what branch %s of repository %s holds: %v", event.Branch, event.Repository, event.Err)
	case drudger.BranchDeleteStep:
		p.out.warn("Could not delete branch %s of repository %s, it stays: %v", event.Branch, event.Repository, event.Err)
	}
}

// describeBase names the commit a ref points at and how old it is. A ref with
// no commit is described by its name alone.
func describeBase(ref string, commit git.Commit) string {
	if commit.SHA == "" {
		return ref
	}
	return fmt.Sprintf("%s at %s, committed %s", ref, git.ShortSHA(commit.SHA), formatAge(time.Since(commit.CommittedAt)))
}

// formatAge renders roughly how long ago a commit was made.
func formatAge(elapsed time.Duration) string {
	switch {
	case elapsed < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(elapsed.Hours()/24))
	}
}

// vendorErrorAdvice tells the user what to do about a refusal.
func vendorErrorAdvice(class task.VendorErrorClass) string {
	switch class {
	case task.VendorErrorAuth:
		return "Log in again, then re-seed the credentials inside sbx. sbx keeps its own copy of the token, and a host login does not refresh it."
	case task.VendorErrorRateLimit:
		return "Run the task again once the vendor lets you through."
	case task.VendorErrorOutage:
		return "Run the task again once the vendor is serving requests."
	default:
		return "Read the error above, fix what it names, then run the task again."
	}
}

// formatArgv renders an argv for display.
func formatArgv(argv []string) string {
	quoted := make([]string, len(argv))
	for index, arg := range argv {
		quoted[index] = strconv.Quote(arg)
	}
	return strings.Join(quoted, " ")
}
