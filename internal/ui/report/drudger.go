package report

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// sessionLevels maps a Session status to the level of the line that records
// it. A status left out records as a success.
var sessionLevels = map[drudger.SessionStatus]Level{
	drudger.StatusNeedsBabysitting: LevelWarning,
	drudger.StatusFuckedUp:         LevelError,
	drudger.StatusNeverGotGoing:    LevelError,
}

// drudgerLines returns the lines of the events the drudger service reports.
func drudgerLines(event any, nameTask TaskNamer) []Line {
	switch event := event.(type) {
	case drudger.DrudgerClaimed:
		return []Line{header("Task %s → Drudger %d (%s)", nameTask(event.Task), event.Slot, event.Sandbox)}
	case drudger.BaseFetchStarted:
		return []Line{step("Fetching %s of %s from %s", event.Branch, event.Repository, event.Remote)}
	case drudger.BaseFetchFailed:
		return []Line{warn("Could not fetch %s of %s: %v", event.Branch, event.Repository, event.Err)}
	case drudger.StaleBaseUsed:
		return []Line{warn("Work on %s is cut from %s", event.Repository, describeBase(event.Ref, event.Commit))}
	case drudger.WorktreeCreationStarted:
		return []Line{step("Creating the workspace of %s at %s", event.Repository, event.Path)}
	case drudger.WorktreeStashed:
		return []Line{done("The workspace of %s held uncommitted changes, they are stashed at %s", event.Repository, git.ShortSHA(event.Commit))}
	case drudger.BranchCheckoutStarted:
		return []Line{step("Putting the workspace on branch %s", event.Branch)}
	case drudger.SandboxLookupStarted:
		return []Line{step("Looking for sandbox %s", event.Sandbox)}
	case drudger.SandboxCreationStarted:
		return []Line{step("Creating sandbox %s, the first one pulls its image, up to %s", event.Sandbox, event.Timeout)}
	case drudger.SandboxReused:
		return []Line{done("Sandbox %s exists, reusing it", event.Sandbox)}
	case drudger.SbxOutput:
		return []Line{detail("%s: %s", event.Binary, event.Line)}
	case drudger.SbxDaemonRetried:
		return []Line{warn("The sbx daemon did not come up, DRUDGE gives it one more try")}
	case drudger.SbxDaemonStarted:
		return []Line{done("The sbx daemon was not running, sbx started it")}
	case drudger.LocalFileCopied:
		return []Line{done("Copied %s of %s into its worktree", event.Path, event.Repository)}
	case drudger.LocalFileMissing:
		return []Line{warn("Did not copy %s of %s, it is not in %s", event.Path, event.Repository, event.Dir)}
	case drudger.SetupUndecided:
		return []Line{setupUndecided(event.Repository)}
	case drudger.SetupCommandStarted:
		return []Line{step("Running setup of %s, up to %s: %s", event.Repository, event.Timeout, event.Command)}
	case drudger.SetupOutput:
		return []Line{detail("%s", event.Line)}
	case drudger.AgentLaunchStarted:
		return []Line{step("Starting the agent, waiting up to %s for its first output", event.GracePeriod)}
	case drudger.AgentKillStarted:
		return []Line{step("Killing the agent in %s, up to %s", event.Sandbox, event.Timeout)}
	case drudger.SandboxStopStarted:
		return []Line{step("Stopping sandbox %s, up to %s", event.Sandbox, event.Timeout)}
	case drudger.AgentLaunched:
		return []Line{
			result(LevelSuccess, "Drudger %s is working on task %s", event.Sandbox, nameTask(event.Task)),
			field("Branch", event.Branch),
			field("Run dir", event.RunDir),
		}
	case drudger.TaskRestarted:
		return []Line{done("Task %s was %q, it starts over", nameTask(event.Task), event.CameFrom)}
	case drudger.RunDescribed:
		return runDescribed(event, nameTask)
	case drudger.DrudgersAboveLimit:
		names := make([]string, 0, len(event.Drudgers))
		for _, above := range event.Drudgers {
			names = append(names, fmt.Sprintf("slot %d (%s)", above.Slot, above.Sandbox))
		}
		return []Line{warn(
			"Project %s has Drudgers above the %s limit of %d, they get no tasks: %s. Raise %s to put them back to work, or nuke them if you are done with them",
			event.ProjectSlug, drudger.MaxConcurrentDrudgersKey, event.Limit, strings.Join(names, ", "), drudger.MaxConcurrentDrudgersKey,
		)}
	case drudger.DrudgerListBehind:
		return []Line{warn("Another drudge command holds the Drudgers of project %s, so this list is what was last written and may be behind", event.ProjectSlug)}
	case drudger.SessionRecordingStarted:
		return []Line{header("Recording the Session of task %s", nameTask(event.Task))}
	case drudger.WorkFoundOnBranch:
		return []Line{done("The agent left %s on branch %s, its work is there", event.Repository, event.Branch)}
	case drudger.RescueBranchCreated:
		return []Line{done("The agent left %s on no branch, its commits are on %s", event.Repository, event.Branch)}
	case drudger.WorkOutOfScope:
		return []Line{warn(
			"The agent made %s in %s, which task %s does not change, they are on branch %s. Run %s to open their pull request",
			git.FormatCommitCount(event.Commits), event.Repository, task.ShortID(event.TaskID), event.Branch, event.Command,
		)}
	case drudger.UncommittedOutOfScope:
		return []Line{warn(
			"The agent left uncommitted changes in %s, which task %s does not list, they are stashed at %s",
			event.Repository, task.ShortID(event.TaskID), git.ShortSHA(event.Commit),
		)}
	case drudger.EmptyBranchDropped:
		return []Line{skip("The agent committed nothing in %s, branch %s is deleted", event.Repository, event.Branch)}
	case drudger.BranchPushStarted:
		return []Line{step("Pushing %s of %s to %s", event.Branch, event.Repository, event.Remote)}
	case drudger.PullRequestOpened:
		return []Line{done("Opened the pull request of %s: %s", event.Repository, event.URL)}
	case drudger.PullRequestDescriptionMissing:
		return []Line{warn("The agent committed in %s and wrote no pull request description, branch %s is not pushed", event.Repository, event.Branch)}
	case drudger.PullRequestRetryStarted:
		return []Line{header("Opening the pull requests of task %s", nameTask(event.Task))}
	case drudger.NoPullRequestLeft:
		return []Line{skip("No pull request description of %s is left, the pull request of branch %s is open or the agent never wrote one", event.Repository, event.Branch)}
	case drudger.PullRequestWithoutLanding:
		return []Line{warn("Task %s has no work in %s, its pull request description stays in the run directory", event.TaskID, event.Repository)}
	case drudger.PullRequestsRetried:
		return []Line{pullRequestsRetried(event, nameTask)}
	case drudger.SessionRecorded:
		return []Line{result(sessionLevel(event.Status), "Task %s %s, it is %s", nameTask(event.Task), event.Status, event.Task.Status)}
	case drudger.DependentsUnblocked:
		lines := make([]Line, 0, len(event.Tasks))
		label := unblockedLabel
		for _, dependent := range event.Tasks {
			lines = append(lines, field(label, nameTask(dependent)))
			label = ""
		}
		return lines
	case drudger.SessionLeftUnrecorded:
		return []Line{skip("Another drudge command is working on task %s, so this check reports the run directory without recording it", event.TaskID)}
	case drudger.RunRefused:
		return []Line{
			result(LevelWarning, "The vendor refused task %s (%s), it is back in %s", nameTask(event.Task), event.Task.VendorErrorClass, event.Task.Status),
			field(adviceLabel, vendorErrorAdvice(event.Task.VendorErrorClass)),
		}
	case drudger.DrudgerNukeStarted:
		return []Line{header("Nuking Drudger %d (%s)", event.Slot, event.Sandbox)}
	case drudger.DrudgerNuked:
		return []Line{result(LevelSuccess, "Drudger %d is gone, sandbox %s was deleted", event.Slot, event.Sandbox)}
	case drudger.SandboxAlreadyGone:
		return []Line{skip("Sandbox %s was already gone", event.Sandbox)}
	case drudger.TaskKilled:
		return []Line{failed("Task %s is %s, its agent was killed with the Drudger", nameTask(event.Task), event.Task.Status)}
	case drudger.BranchOfUnknownRepositoryKept:
		return []Line{skip("Branch %s stays, project %s records no repository %s", event.Branch, event.ProjectSlug, event.Repository)}
	case drudger.BranchWithCommitsKept:
		return []Line{skip("Branch %s of repository %s holds commits, it stays", event.Branch, event.Repository)}
	case drudger.EmptyBranchRemoved:
		return []Line{done("Branch %s of repository %s held nothing, it is deleted", event.Branch, event.Repository)}
	case drudger.HealthRecordFailed:
		return healthRecordFailed(event)
	case drudger.IdleWorkspaceParkFailed:
		return idleWorkspaceParkFailed(event)
	case drudger.RunCloseOutFailed:
		return runCloseOutFailed(event)
	case drudger.PullRequestFailed:
		return pullRequestFailed(event)
	case drudger.WorkspaceNukeFailed:
		return workspaceNukeFailed(event)
	case drudger.BranchCleanupFailed:
		return branchCleanupFailed(event)
	case drudger.UnblockedLookupFailed:
		return []Line{warn("Task %s is done, but the tasks it unblocked could not be worked out: %v", event.TaskID, event.Err)}
	case drudger.DrudgerReleaseFailed:
		return []Line{warn("Drudger %d of project %s stays claimed for a run that never started: %v", event.Slot, event.ProjectSlug, event.Err)}
	case drudger.SessionIDReadFailed:
		return []Line{warn("%v, the task is recorded without a session id", event.Err)}
	default:
		return nil
	}
}

// setupUndecided warns about a repository with no setup key and names the
// command that writes one.
func setupUndecided(repository string) Line {
	return warn("Repository %s has no setup in the local config, run %s to detect it", repository, drudger.DetectCommand)
}

func runDescribed(event drudger.RunDescribed, nameTask TaskNamer) []Line {
	lines := []Line{
		header("Dry run of task %s on Drudger %d (%s)", nameTask(event.Task), event.Slot, event.Sandbox),
		field("Prompt from", event.PromptSource),
	}
	if event.PullRequestProvider != "" {
		lines = append(lines, field("Pull requests", fmt.Sprintf("opened on %s", event.PullRequestProvider)))
	}
	if len(event.EnvNames) > 0 {
		lines = append(lines, field("Environment", strings.Join(event.EnvNames, ", ")))
	}
	for _, name := range event.UnsetEnvNames {
		lines = append(lines, warn("%s is not set in this environment, a launch stops on it", name))
	}
	for _, repository := range event.UndecidedSetups {
		lines = append(lines, setupUndecided(repository))
	}
	if event.HasLiveAgent {
		lines = append(lines, warn("Sandbox %s has an agent running, a launch stops on it. Run %s %d to kill it", event.Sandbox, drudger.NukeCommand, event.Slot))
	}
	if event.AgentCheckErr != nil {
		lines = append(lines, warn("Could not look for a running agent in sandbox %s: %v", event.Sandbox, event.AgentCheckErr))
	}
	lines = append(lines, block(event.Prompt))
	if len(event.Copies) > 0 {
		copies := make([]string, 0, len(event.Copies))
		for _, copied := range event.Copies {
			copies = append(copies, fmt.Sprintf("%s  %s", copied.Repository, copied.Path))
		}
		lines = append(lines, field("Copies", ""), block(strings.Join(copies, "\n")))
	}
	commands := make([]string, 0, len(event.Commands))
	for _, argv := range event.Commands {
		commands = append(commands, formatArgv(argv))
	}
	return append(lines,
		field("Commands", ""),
		block(strings.Join(commands, "\n")),
		result(LevelSkipped, "Nothing ran, it was a dry run"),
	)
}

func sessionLevel(status drudger.SessionStatus) Level {
	if level, ok := sessionLevels[status]; ok {
		return level
	}
	return LevelSuccess
}

// pullRequestsRetried closes a retry with how many of the pull requests it
// tried are open now.
func pullRequestsRetried(event drudger.PullRequestsRetried, nameTask TaskNamer) Line {
	switch {
	case event.Tried == 0:
		return result(LevelSkipped, "Task %s has no pull requests left to open", nameTask(event.Task))
	case event.Opened == event.Tried:
		return result(LevelSuccess, "Opened %d pull requests of task %s", event.Opened, nameTask(event.Task))
	default:
		return result(LevelWarning, "Opened %d of %d pull requests of task %s", event.Opened, event.Tried, nameTask(event.Task))
	}
}

func healthRecordFailed(event drudger.HealthRecordFailed) []Line {
	switch event.Part {
	case drudger.SandboxPart:
		return []Line{warn("The sandbox of Drudger %d of project %s is %s, but that could not be recorded: %v", event.Slot, event.ProjectSlug, event.Health, event.Err)}
	case drudger.WorkspacePart:
		return []Line{warn("The workspace of Drudger %d of project %s is %s, but that could not be recorded: %v", event.Slot, event.ProjectSlug, event.Health, event.Err)}
	case drudger.AgentPart:
		return []Line{warn("The agent that ran task %s of project %s is %s, but that could not be recorded: %v", event.TaskID, event.ProjectSlug, event.Health, event.Err)}
	default:
		return nil
	}
}

func idleWorkspaceParkFailed(event drudger.IdleWorkspaceParkFailed) []Line {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		return []Line{warn("Drudger %d of project %s holds no task, but the workspace it works in could not be read: %v", event.Slot, event.ProjectSlug, event.Err)}
	case drudger.WorkspaceParkStep:
		return []Line{warn("Drudger %d of project %s holds no task, but its workspace could not be parked: %v", event.Slot, event.ProjectSlug, event.Err)}
	default:
		return nil
	}
}

func runCloseOutFailed(event drudger.RunCloseOutFailed) []Line {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		return []Line{warn("The Session of task %s is over, but the workspace it ran in could not be read: %v", event.TaskID, event.Err)}
	case drudger.WorkspaceParkStep:
		return []Line{warn("The Session of task %s is over, but the workspace it ran in could not be parked: %v", event.TaskID, event.Err)}
	case drudger.RepositoryCloseOutStep:
		return []Line{warn("The Session of task %s is over, but where its work in repository %s is could not be worked out: %v", event.TaskID, event.Repository, event.Err)}
	default:
		return nil
	}
}

func pullRequestFailed(event drudger.PullRequestFailed) []Line {
	switch event.Step {
	case drudger.DescriptionMoveStep:
		return []Line{warn("Could not move the pull request description of %s into the run directory of task %s, no pull request is opened: %v", event.Repository, event.TaskID, event.Err)}
	case drudger.DescriptionReadStep:
		return []Line{warn("Could not read the pull request description of %s in the run directory of task %s, no pull request is opened: %v", event.Repository, event.TaskID, event.Err)}
	case drudger.BranchPushStep:
		return []Line{warn("Could not push the branch of %s, its pull request description stays in the run directory of task %s: %v", event.Repository, event.TaskID, event.Err)}
	case drudger.PullRequestOpenStep:
		return []Line{warn("Could not open the pull request of %s, its description stays in the run directory of task %s: %v", event.Repository, event.TaskID, event.Err)}
	case drudger.DescriptionRemoveStep:
		return []Line{warn("The pull request of %s is open, but its description could not be deleted from the run directory of task %s: %v", event.Repository, event.TaskID, event.Err)}
	default:
		return nil
	}
}

func workspaceNukeFailed(event drudger.WorkspaceNukeFailed) []Line {
	switch event.Step {
	case drudger.WorkspaceReadStep:
		return []Line{warn("Drudger %d of project %s is being nuked, but the workspace it works in could not be read: %v", event.Slot, event.ProjectSlug, event.Err)}
	case drudger.WorktreeRemovalStep:
		return []Line{warn("Drudger %d of project %s is being nuked, but its worktree of repository %s could not be taken out: %v", event.Slot, event.ProjectSlug, event.Repository, event.Err)}
	default:
		return nil
	}
}

func branchCleanupFailed(event drudger.BranchCleanupFailed) []Line {
	switch event.Step {
	case drudger.RepositoryReadStep:
		return []Line{warn("Could not read repository %s, branch %s stays: %v", event.Repository, event.Branch, event.Err)}
	case drudger.BranchReadStep:
		return []Line{warn("Could not read branch %s of repository %s: %v", event.Branch, event.Repository, event.Err)}
	case drudger.BranchInspectStep:
		return []Line{warn("Could not read what branch %s of repository %s holds: %v", event.Branch, event.Repository, event.Err)}
	case drudger.BranchDeleteStep:
		return []Line{warn("Could not delete branch %s of repository %s, it stays: %v", event.Branch, event.Repository, event.Err)}
	default:
		return nil
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
