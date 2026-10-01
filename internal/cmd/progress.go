package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
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
	out *printer.Printer
}

// newCLIProgress builds a progress port that prints through out.
func newCLIProgress(out *printer.Printer) *cliProgress {
	return &cliProgress{out: out}
}

func (p *cliProgress) Report(event any) {
	switch event := event.(type) {
	case task.TaskCreated:
		p.out.Done("Created task %s", p.out.Task(event.Task))
	case task.TaskRemovalDeclined:
		p.out.Skip("Left task %s alone", p.out.Task(event.Task))
	case task.TaskRemovalStarted:
		p.out.Header("Removing task %s", p.out.Task(event.Task))
	case task.RunDirectoryRemoved:
		p.out.Done("Removed its run directory")
	case task.TaskRemoved:
		p.out.Result("Removed task %s", p.out.Task(event.Task))
	case task.TasksUnblocked:
		p.out.Done("Took it off the blockers of %s", task.FormatTaskCount(event.Count))
	case task.TasksUngrouped:
		p.out.Done("Ungrouped %s that belonged to it", task.FormatTaskCount(event.Count))
	case task.BranchesCleanupFailed:
		p.out.Warn("Task %s is removed, but the branches it left could not be cleaned up: %v", event.TaskID, event.Err)
	case task.TaskUnlinkFailed:
		p.warnTaskUnlinkFailed(event)
	case task.TaskMarkedDone:
		p.out.Done("Task %s is %q", p.out.Task(event.Task), event.Task.Status)
	case task.TaskEdited:
		p.out.Done("Updated task %s, it is now %q", p.out.Task(event.Task), event.Task.Status)
	case project.ProjectCreated:
		// Project init prints the result line itself, after the warnings
		// about its repositories.
	case project.ProjectRenamed:
		p.out.Done("Renamed project %s from %q to %q", event.Slug, event.OldName, event.NewName)
	case project.ProjectRemoved:
		p.out.Done("Removed project %s", event.Name)
	case project.ProjectAlreadyGone:
		p.out.Skip("Project %s was already gone", event.Slug)
	case release.DownloadStarted:
		p.out.Step("Downloading %s (%s)", event.ArchiveName, event.Version)
	default:
		p.reportDrudger(event)
	}
}

func (p *cliProgress) warnTaskUnlinkFailed(event task.TaskUnlinkFailed) {
	if event.IsHeld {
		p.out.Warn("Task %s is removed, but another drudge command is working on task %s, which still names it", event.RemovedID, event.LinkedID)
		return
	}
	p.out.Warn("Task %s is removed, but task %s still names it: %v", event.RemovedID, event.LinkedID, event.Err)
}

// reportDrudger renders the events the drudger service reports.
func (p *cliProgress) reportDrudger(event any) {
	switch event := event.(type) {
	case drudger.DrudgerClaimed:
		p.out.Header("Task %s → Drudger %d (%s)", p.out.Task(event.Task), event.Slot, event.Sandbox)
	case drudger.BaseFetchStarted:
		p.out.Step("Fetching %s of %s from %s", event.Branch, event.Repository, event.Remote)
	case drudger.BaseFetchFailed:
		p.out.Warn("Could not fetch %s of %s: %v", event.Branch, event.Repository, event.Err)
	case drudger.StaleBaseUsed:
		p.out.Warn("Work on %s is cut from %s", event.Repository, describeBase(event.Ref, event.Commit))
	case drudger.WorktreeCreationStarted:
		p.out.Step("Creating the workspace of %s at %s", event.Repository, event.Path)
	case drudger.WorktreeStashed:
		p.out.Done("The workspace of %s held uncommitted changes, they are stashed at %s", event.Repository, git.ShortSHA(event.Commit))
	case drudger.BranchCheckoutStarted:
		p.out.Step("Putting the workspace on branch %s", event.Branch)
	case drudger.SandboxLookupStarted:
		p.out.Step("Looking for sandbox %s", event.Sandbox)
	case drudger.SandboxCreationStarted:
		p.out.Step("Creating sandbox %s, the first one pulls its image, up to %s", event.Sandbox, event.Timeout)
	case drudger.SandboxReused:
		p.out.Done("Sandbox %s exists, reusing it", event.Sandbox)
	case drudger.SbxOutput:
		p.out.Detail("%s: %s", event.Binary, event.Line)
	case drudger.SbxDaemonRetried:
		p.out.Warn("The sbx daemon did not come up, DRUDGE gives it one more try")
	case drudger.SbxDaemonStarted:
		p.out.Done("The sbx daemon was not running, sbx started it")
	case drudger.AgentLaunchStarted:
		p.out.Step("Starting the agent, waiting up to %s for its first output", event.GracePeriod)
	case drudger.AgentLaunched:
		p.out.Result("Drudger %s is working on task %s", event.Sandbox, p.out.Task(event.Task))
		p.out.Field("Branch", event.Branch)
		p.out.Field("Run dir", event.RunDir)
		p.out.Flush()
	case drudger.TaskRestarted:
		p.out.Done("Task %s was %q, it starts over", p.out.Task(event.Task), event.CameFrom)
	case drudger.RunDescribed:
		commands := make([]string, 0, len(event.Commands))
		for _, argv := range event.Commands {
			commands = append(commands, formatArgv(argv))
		}
		p.out.Header("Dry run of task %s on Drudger %d (%s)", p.out.Task(event.Task), event.Slot, event.Sandbox)
		p.out.Field("Prompt from", event.PromptSource)
		p.out.Block(event.Prompt)
		p.out.Field("Commands", "")
		p.out.Block(strings.Join(commands, "\n"))
		p.out.SkipResult("Nothing ran, it was a dry run")
	case drudger.DrudgersAboveLimit:
		names := make([]string, 0, len(event.Drudgers))
		for _, above := range event.Drudgers {
			names = append(names, fmt.Sprintf("slot %d (%s)", above.Slot, above.Sandbox))
		}
		p.out.Warn(
			"Project %s has Drudgers above the %s limit of %d, they get no tasks: %s. Raise %s to put them back to work, or nuke them if you are done with them",
			event.ProjectSlug, drudger.MaxConcurrentDrudgersKey, event.Limit, strings.Join(names, ", "), drudger.MaxConcurrentDrudgersKey,
		)
	case drudger.DrudgerListBehind:
		p.out.Warn("Another drudge command holds the Drudgers of project %s, so this list is what was last written and may be behind", event.ProjectSlug)
	case drudger.SessionRecordingStarted:
		p.out.Header("Recording the Session of task %s", p.out.Task(event.Task))
	case drudger.WorkFoundOnBranch:
		p.out.Done("The agent left %s on branch %s, its work is there", event.Repository, event.Branch)
	case drudger.RescueBranchCreated:
		p.out.Done("The agent left %s on no branch, its commits are on %s", event.Repository, event.Branch)
	case drudger.EmptyBranchDropped:
		p.out.Skip("The agent committed nothing in %s, branch %s is deleted", event.Repository, event.Branch)
	case drudger.SessionRecorded:
		p.sessionResult(event.Status)("Task %s %s, it is %s", p.out.Task(event.Task), event.Status, event.Task.Status)
	case drudger.DependentsUnblocked:
		label := unblockedLabel
		for _, dependent := range event.Tasks {
			p.out.Field(label, p.out.Task(dependent))
			label = ""
		}
		p.out.Flush()
	case drudger.SessionLeftUnrecorded:
		p.out.Skip("Another drudge command is working on task %s, so this check reports the run directory without recording it", event.TaskID)
	case drudger.RunRefused:
		p.out.ResultWarn("The vendor refused task %s (%s), it is back in %s", p.out.Task(event.Task), event.Task.VendorErrorClass, event.Task.Status)
		p.out.Field(adviceLabel, vendorErrorAdvice(event.Task.VendorErrorClass))
		p.out.Flush()
	case drudger.DrudgerNukeStarted:
		p.out.Header("Nuking Drudger %d (%s)", event.Slot, event.Sandbox)
	case drudger.DrudgerNuked:
		p.out.Result("Drudger %d is gone, sandbox %s was deleted", event.Slot, event.Sandbox)
	case drudger.SandboxAlreadyGone:
		p.out.Skip("Sandbox %s was already gone", event.Sandbox)
	case drudger.TaskKilled:
		p.out.Failed("Task %s is %s, its agent was killed with the Drudger", p.out.Task(event.Task), event.Task.Status)
	case drudger.BranchOfUnknownRepositoryKept:
		p.out.Skip("Branch %s stays, project %s records no repository %s", event.Branch, event.ProjectSlug, event.Repository)
	case drudger.BranchWithCommitsKept:
		p.out.Skip("Branch %s of repository %s holds commits, it stays", event.Branch, event.Repository)
	case drudger.EmptyBranchRemoved:
		p.out.Done("Branch %s of repository %s held nothing, it is deleted", event.Branch, event.Repository)
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
		p.out.Warn("Task %s is done, but the tasks it unblocked could not be worked out: %v", event.TaskID, event.Err)
	case drudger.DrudgerReleaseFailed:
		p.out.Warn("Drudger %d of project %s stays claimed for a run that never started: %v", event.Slot, event.ProjectSlug, event.Err)
	case drudger.SessionIDReadFailed:
		p.out.Warn("%v, the task is recorded without a session id", event.Err)
	}
}

// sessionResult picks the result line of a recorded Session by the role its
// status prints in.
func (p *cliProgress) sessionResult(status drudger.SessionStatus) func(format string, args ...any) {
	switch sessionStatusRoles[status] {
	case theme.RoleWarning:
		return p.out.ResultWarn
	case theme.RoleError:
		return p.out.ResultFailed
	default:
		return p.out.Result
	}
}

func (p *cliProgress) warnHealthRecordFailed(event drudger.HealthRecordFailed) {
	switch event.Part {
	case drudger.SandboxPart:
		p.out.Warn("The sandbox of Drudger %d of project %s is %s, but that could not be recorded: %v", event.Slot, event.ProjectSlug, event.Health, event.Err)
	case drudger.WorkspacePart:
		p.out.Warn("The workspace of Drudger %d of project %s is %s, but that could not be recorded: %v", event.Slot, event.ProjectSlug, event.Health, event.Err)
	case drudger.AgentPart:
		p.out.Warn("The agent that ran task %s of project %s is %s, but that could not be recorded: %v", event.TaskID, event.ProjectSlug, event.Health, event.Err)
	}
}

func (p *cliProgress) warnIdleWorkspaceParkFailed(event drudger.IdleWorkspaceParkFailed) {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		p.out.Warn("Drudger %d of project %s holds no task, but the workspace it works in could not be read: %v", event.Slot, event.ProjectSlug, event.Err)
	case drudger.WorkspaceParkStep:
		p.out.Warn("Drudger %d of project %s holds no task, but its workspace could not be parked: %v", event.Slot, event.ProjectSlug, event.Err)
	}
}

func (p *cliProgress) warnRunCloseOutFailed(event drudger.RunCloseOutFailed) {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		p.out.Warn("The Session of task %s is over, but the workspace it ran in could not be read: %v", event.TaskID, event.Err)
	case drudger.WorkspaceParkStep:
		p.out.Warn("The Session of task %s is over, but the workspace it ran in could not be parked: %v", event.TaskID, event.Err)
	case drudger.RepositoryCloseOutStep:
		p.out.Warn("The Session of task %s is over, but where its work in repository %s is could not be worked out: %v", event.TaskID, event.Repository, event.Err)
	}
}

func (p *cliProgress) warnWorkspaceNukeFailed(event drudger.WorkspaceNukeFailed) {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		p.out.Warn("Drudger %d of project %s is being nuked, but the workspace it works in could not be read: %v", event.Slot, event.ProjectSlug, event.Err)
	case drudger.WorktreeRemovalStep:
		p.out.Warn("Drudger %d of project %s is being nuked, but its worktree of repository %s could not be taken out: %v", event.Slot, event.ProjectSlug, event.Repository, event.Err)
	}
}

func (p *cliProgress) warnBranchCleanupFailed(event drudger.BranchCleanupFailed) {
	switch event.Step {
	case drudger.RepositoryReadStep:
		p.out.Warn("Could not read repository %s, branch %s stays: %v", event.Repository, event.Branch, event.Err)
	case drudger.BranchReadStep:
		p.out.Warn("Could not read branch %s of repository %s: %v", event.Branch, event.Repository, event.Err)
	case drudger.BranchInspectStep:
		p.out.Warn("Could not read what branch %s of repository %s holds: %v", event.Branch, event.Repository, event.Err)
	case drudger.BranchDeleteStep:
		p.out.Warn("Could not delete branch %s of repository %s, it stays: %v", event.Branch, event.Repository, event.Err)
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
