package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// unblockedTaskLine lays out one task a finished task unblocked.
const unblockedTaskLine = "  %s  %s"

// cliProgress renders the events a domain service reports as the lines drg
// prints today. A later change can give these lines a different layout and
// colors without touching a service.
type cliProgress struct {
	log *common.Logger
}

// newCLIProgress builds a progress port that prints through log.
func newCLIProgress(log *common.Logger) *cliProgress {
	return &cliProgress{log: log}
}

func (p *cliProgress) Report(event any) {
	switch event := event.(type) {
	case task.TaskCreated:
		p.log.Info("Created task [%s] %s", event.Task.ID, event.Task.Title)
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
	case task.TaskMarkedDone:
		p.log.Info("Task [%s] %s is %q", event.Task.ID, event.Task.Title, event.Task.Status)
	case task.TaskEdited:
		p.log.Info("Updated task [%s] %s, it is now %q", event.Task.ID, event.Task.Title, event.Task.Status)
	case project.ProjectCreated:
		p.log.Info("Created project %s", event.Project.Name)
	case project.ProjectRenamed:
		p.log.Info("Renamed project %s from %q to %q", event.Slug, event.OldName, event.NewName)
	case release.DownloadStarted:
		p.log.Info("Downloading %s (%s)", event.ArchiveName, event.Version)
	default:
		p.reportDrudger(event)
	}
}

// reportDrudger renders the events the drudger service reports.
func (p *cliProgress) reportDrudger(event any) {
	switch event := event.(type) {
	case drudger.DrudgerClaimed:
		p.log.Info("Task [%s] %s goes to Drudger %d (%s)", event.Task.ID, event.Task.Title, event.Slot, event.Sandbox)
	case drudger.BaseFetchStarted:
		p.log.Info("Fetching %s of repository %s from %s", event.Branch, event.Repository, event.Remote)
	case drudger.WorktreeCreationStarted:
		p.log.Info("Creating the workspace of repository %s at %s", event.Repository, event.Path)
	case drudger.WorktreeStashed:
		p.log.Info("Repository %s held uncommitted changes, they are stashed at %s", event.Repository, git.ShortSHA(event.Commit))
	case drudger.BranchCheckoutStarted:
		p.log.Info("Putting the workspace on branch %s", event.Branch)
	case drudger.SandboxLookupStarted:
		p.log.Info("Looking for sandbox %s", event.Sandbox)
	case drudger.SandboxCreationStarted:
		p.log.Info(
			"Sandbox %s does not exist yet, creating it. The first sandbox of a harness pulls its image, which takes minutes. DRUDGE waits up to %s",
			event.Sandbox, event.Timeout,
		)
	case drudger.SandboxReused:
		p.log.Info("Sandbox %s exists, reusing it", event.Sandbox)
	case drudger.SbxOutput:
		p.log.Info("%s: %s", event.Binary, event.Line)
	case drudger.SbxDaemonRetried:
		p.log.Info("The sbx daemon did not come up, DRUDGE gives it one more try")
	case drudger.SbxDaemonStarted:
		p.log.Info("The sbx daemon was not running, sbx has just started it")
	case drudger.AgentLaunchStarted:
		p.log.Info("Starting the agent in sandbox %s and waiting up to %s for its first output", event.Sandbox, event.GracePeriod)
	case drudger.AgentLaunched:
		p.log.Info("Drudger %s is working on task [%s] %s", event.Sandbox, event.Task.ID, event.Task.Title)
		p.log.Info("Branch: %s", event.Branch)
		p.log.Info("Run directory: %s", event.RunDir)
	case drudger.TaskRestarted:
		p.log.Info("Task [%s] %s was %q, its previous run is cleared and it starts over", event.Task.ID, event.Task.Title, event.CameFrom)
	case drudger.RunDescribed:
		commands := make([]string, 0, len(event.Commands))
		for _, argv := range event.Commands {
			commands = append(commands, formatArgv(argv))
		}
		p.log.Info("Drudger %d (%s) for task [%s] %s", event.Slot, event.Sandbox, event.Task.ID, event.Task.Title)
		p.log.Info("Prompt (from %s):\n\n%s", event.PromptSource, event.Prompt)
		p.log.Info("Commands:\n\n%s", strings.Join(commands, "\n"))
	case drudger.DrudgersAboveLimit:
		names := make([]string, 0, len(event.Drudgers))
		for _, above := range event.Drudgers {
			names = append(names, fmt.Sprintf("slot %d (%s)", above.Slot, above.Sandbox))
		}
		p.log.Info("Project %s has Drudgers above the %s limit of %d: %s", event.ProjectSlug, drudger.MaxConcurrentDrudgersKey, event.Limit, strings.Join(names, ", "))
		p.log.Info("They are left alone and the task was not assigned to them. Raise %s to put them back to work, or nuke them if you are done with them.", drudger.MaxConcurrentDrudgersKey)
	case drudger.DrudgerListBehind:
		p.log.Info("Another drudge command holds the Drudgers of project %s, so this list is what was last written and may be behind", event.ProjectSlug)
	case drudger.SessionRecorded:
		p.log.Info("Task [%s] %s is %s, its Session is over", event.Task.ID, event.Task.Title, event.Task.Status)
	case drudger.SessionLeftUnrecorded:
		p.log.Info("Another drudge command is working on task %s, so this check reports the run directory without recording it", event.TaskID)
	case drudger.RunRefused:
		p.log.Info("The vendor refused the agent on task [%s] %s (%s): %s", event.Task.ID, event.Task.Title, event.Task.VendorErrorClass, event.Task.VendorError)
		p.log.Info("Nothing ran, so the task is back in %q.", event.Task.Status)
		p.log.Info("%s", vendorErrorAdvice(event.Task.VendorErrorClass))
	case drudger.WorkFoundOnBranch:
		p.log.Info("The agent left repository %s on branch %s, which is where its work is", event.Repository, event.Branch)
	case drudger.RescueBranchCreated:
		p.log.Info("The agent left repository %s on no branch, its commits are on %s", event.Repository, event.Branch)
	case drudger.EmptyBranchDropped:
		p.log.Info("The agent committed nothing in repository %s, so branch %s is deleted", event.Repository, event.Branch)
	case drudger.DependentsUnblocked:
		p.log.Info("It unblocked %s:", task.FormatTaskCount(len(event.Tasks)))
		for _, dependent := range event.Tasks {
			p.log.Info(unblockedTaskLine, task.ShortID(dependent.ID), dependent.Title)
		}
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
