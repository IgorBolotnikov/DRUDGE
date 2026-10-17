package drudger

import (
	"fmt"
	"maps"
	"slices"

	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// rescueBranchSuffix ends the name of a branch close-out puts on commits no
// branch reaches.
const rescueBranchSuffix = "-rescue"

// pullRequestCommand is what a user runs to open the pull requests of a task
// that close-out left behind.
const pullRequestCommand = "drg task pr"

// WorkFoundOnBranch reports a repository an agent left on a branch other than
// the one its handover made. The branch holds the work of the run.
type WorkFoundOnBranch struct {
	Repository string
	Branch     string
}

// RescueBranchCreated reports a branch close-out put on commits an agent left
// on no branch.
type RescueBranchCreated struct {
	Repository string
	Branch     string
}

// EmptyBranchDropped reports a branch close-out deleted because the agent
// committed nothing on it.
type EmptyBranchDropped struct {
	Repository string
	Branch     string
}

// WorkOutOfScope reports commits an agent made in a repository its task does
// not change. Their pull request is not opened, and Command is what a user
// runs to open it.
type WorkOutOfScope struct {
	TaskID     task.TaskID
	Repository string
	Branch     string
	Commits    int
	Command    string
}

// RunCloseOutFailed reports a step of closing out a finished run that failed.
// Repository is set for the close-out of one repository.
type RunCloseOutFailed struct {
	TaskID     task.TaskID
	Step       WorkspaceStep
	Repository string
	Err        error
}

// finishRun records where the work of a finished run landed, opens its pull
// requests and parks the workspace it ran in. The caller writes the task back.
// outOfScope names the repositories the handover put on no branch.
//
// Close-out runs first. It reads where the agent left each worktree, and
// parking moves them. Pull requests are opened only for a Session that got
// shit done, with pull requests on.
//
// A workspace it cannot read is reported and the task keeps what the handover
// recorded. Reading git and opening pull requests never cost the outcome of
// the run.
func (service *DrudgerService) finishRun(projectSlug string, finished *task.Task, status SessionStatus, outOfScope []string) {
	space, err := service.workspaceOfTask(projectSlug, finished.ID)
	if err != nil {
		service.progress.Report(RunCloseOutFailed{TaskID: finished.ID, Step: WorkspaceReadStep, Err: err})
		return
	}

	service.closeOutRun(space, finished, outOfScope)

	if status == StatusGotShitDone && service.remote != nil {
		service.openPullRequests(space, finished, outOfScope)
	}

	if err := service.parkWorkspace(space, finished); err != nil {
		service.progress.Report(RunCloseOutFailed{TaskID: finished.ID, Step: WorkspaceParkStep, Err: err})
	}
}

// outOfScopeRepositories names the repositories a task has a landing with no
// branch in. The handover leaves a repository the task does not change on no
// branch. Close-out reads the landings and not the list on the task, so an
// edit of the list during a run takes effect on the next run.
func outOfScopeRepositories(finished *task.Task) []string {
	var outOfScope []string
	for _, repository := range slices.Sorted(maps.Keys(finished.Landings)) {
		if !finished.Landings[repository].HasBranch() {
			outOfScope = append(outOfScope, repository)
		}
	}
	return outOfScope
}

// handedOverBranch is the branch the handover put the repositories of a task
// on. It is empty when every landing has no branch.
func handedOverBranch(handedOver *task.Task) string {
	for _, repository := range slices.Sorted(maps.Keys(handedOver.Landings)) {
		if landing := handedOver.Landings[repository]; landing.HasBranch() {
			return landing.Branch
		}
	}
	return ""
}

// closeOutRun records where the work of the run is in each repository of the
// workspace. A repository the task has no landing for is skipped.
//
// A repository that cannot be read is reported and the rest are still closed
// out.
func (service *DrudgerService) closeOutRun(space slotWorkspace, finished *task.Task, outOfScope []string) {
	// Close-out moves landings to other branches, so the handover branch is
	// read before any of them.
	taskBranch := handedOverBranch(finished)

	for _, repository := range space.Repositories {
		landing, hasLanding := finished.Landings[repository.Name]
		if !hasLanding {
			continue
		}

		var err error
		if slices.Contains(outOfScope, repository.Name) {
			err = service.closeOutOutOfScope(finished, repository, landing, taskBranch)
		} else {
			err = service.closeOutRepository(finished, repository, landing)
		}
		if err != nil {
			service.progress.Report(RunCloseOutFailed{TaskID: finished.ID, Step: RepositoryCloseOutStep, Repository: repository.Name, Err: err})
		}
	}
}

// workspaceOfTask returns the workspace the agent of a task worked in. The
// slot is what ties a task to its worktrees, so a task no Drudger holds has no
// workspace to read.
func (service *DrudgerService) workspaceOfTask(projectSlug string, taskID task.TaskID) (slotWorkspace, error) {
	layout, err := service.layout()
	if err != nil {
		return slotWorkspace{}, err
	}

	drudgers, err := service.drudgers.ListDrudgers(projectSlug)
	if err != nil {
		return slotWorkspace{}, err
	}

	holder := drudgerHoldingTask(drudgers, taskID)
	if holder == nil {
		return slotWorkspace{}, fmt.Errorf("no Drudger of project %s holds task %s", projectSlug, taskID)
	}
	return service.resolveWorkspace(layout, holder)
}

// closeOutRepository records what one repository holds now that the run is
// over.
func (service *DrudgerService) closeOutRepository(finished *task.Task, repository repositoryWorktree, landing task.Landing) error {
	branch, err := service.branchOfWork(repository, landing)
	if err != nil {
		return err
	}

	tip, err := service.gitOps.ResolveCommit(repository.Worktree, branch)
	if err != nil {
		return err
	}

	commits, err := service.gitOps.CommitCount(repository.Worktree, landing.Base, tip.SHA)
	if err != nil {
		return err
	}
	if commits == 0 {
		finished.DropLanding(repository.Name)
		return service.dropEmptyBranch(repository, landing, tip.SHA)
	}

	if branch != landing.Branch {
		service.progress.Report(WorkFoundOnBranch{Repository: repository.Name, Branch: branch})
	}

	finished.RecordLanding(repository.Name, task.Landing{Branch: branch, Base: landing.Base, Head: tip.SHA, Commits: commits})
	return nil
}

// closeOutOutOfScope records the commits an agent made in a repository its
// task does not change. A HEAD holding nothing beyond the base loses its
// landing and is not reported. Commits are reported along with the command
// that opens their pull request.
func (service *DrudgerService) closeOutOutOfScope(finished *task.Task, repository repositoryWorktree, landing task.Landing, taskBranch string) error {
	head, err := service.gitOps.ResolveHeadCommit(repository.Worktree)
	if err != nil {
		return err
	}

	commits, err := service.gitOps.CommitCount(repository.Worktree, landing.Base, head.SHA)
	if err != nil {
		return err
	}
	if commits == 0 {
		finished.DropLanding(repository.Name)
		return nil
	}

	branch, err := service.gitOps.CurrentBranch(repository.Worktree)
	if err != nil {
		return err
	}
	if branch == "" {
		branch, err = service.branchDetachedWork(repository, taskBranch, head.SHA)
		if err != nil {
			return err
		}
	}

	finished.RecordLanding(repository.Name, task.Landing{Branch: branch, Base: landing.Base, Head: head.SHA, Commits: commits})
	service.progress.Report(WorkOutOfScope{
		TaskID:     finished.ID,
		Repository: repository.Name,
		Branch:     branch,
		Commits:    commits,
		Command:    fmt.Sprintf("%s %s", pullRequestCommand, task.ShortID(finished.ID)),
	})
	return nil
}

// branchDetachedWork puts the task branch on the commits of a detached HEAD
// in a repository the task does not change. A repository that already has a
// branch of that name gets a rescue branch.
func (service *DrudgerService) branchDetachedWork(repository repositoryWorktree, taskBranch string, head string) (string, error) {
	if taskBranch == "" {
		return "", fmt.Errorf("the handover put no repository on a branch, so the commits in repository %s get no branch", repository.Name)
	}

	isTaken, err := service.gitOps.BranchExists(repository.Worktree, taskBranch)
	if err != nil {
		return "", err
	}
	if isTaken {
		return service.rescueBranch(repository, taskBranch, head)
	}

	if err := service.gitOps.CreateBranch(repository.Worktree, taskBranch, head); err != nil {
		return "", err
	}
	service.progress.Report(RescueBranchCreated{Repository: repository.Name, Branch: taskBranch})
	return taskBranch, nil
}

// branchOfWork names the branch holding the work of a run. An agent that ended
// on a branch gets that branch. One that left HEAD detached on commits of its
// own gets a branch reaching them, and commits no branch reaches get a branch
// made for them.
//
// A detached HEAD holding nothing beyond the base keeps the branch the
// handover made, so a worktree detached after the run still reports what that
// branch holds.
func (service *DrudgerService) branchOfWork(repository repositoryWorktree, landing task.Landing) (string, error) {
	current, err := service.gitOps.CurrentBranch(repository.Worktree)
	if err != nil {
		return "", err
	}
	if current != "" {
		return current, nil
	}

	head, err := service.gitOps.ResolveHeadCommit(repository.Worktree)
	if err != nil {
		return "", err
	}

	detached, err := service.gitOps.CommitCount(repository.Worktree, landing.Base, head.SHA)
	if err != nil {
		return "", err
	}
	if detached == 0 {
		return landing.Branch, nil
	}

	branches, err := service.gitOps.BranchesContaining(repository.Worktree, head.SHA)
	if err != nil {
		return "", err
	}
	if slices.Contains(branches, landing.Branch) {
		return landing.Branch, nil
	}
	if len(branches) > 0 {
		return branches[0], nil
	}
	return service.rescueBranch(repository, landing.Branch, head.SHA)
}

// rescueBranch puts a branch on the commits of a detached HEAD that no branch
// reaches. It takes the first free name in the series, so a rerun that detaches
// again leaves the branch of the attempt before it alone.
func (service *DrudgerService) rescueBranch(repository repositoryWorktree, handedOver string, head string) (string, error) {
	wanted := handedOver + rescueBranchSuffix

	for attempt := 1; attempt <= branchAttempts; attempt++ {
		candidate := attemptBranch(wanted, attempt)

		isTaken, err := service.gitOps.BranchExists(repository.Worktree, candidate)
		if err != nil {
			return "", err
		}
		if isTaken {
			continue
		}

		if err := service.gitOps.CreateBranch(repository.Worktree, candidate, head); err != nil {
			return "", err
		}
		service.progress.Report(RescueBranchCreated{Repository: repository.Name, Branch: candidate})
		return candidate, nil
	}

	return "", fmt.Errorf("repository %s already has %d branches named after %s, merge or delete them", repository.Name, branchAttempts, wanted)
}

// dropEmptyBranch deletes the branch a handover made in a repository the run
// left no commits in. A worktree still on that branch is detached where it
// stands first, because git refuses to delete a branch a work tree holds.
//
// A branch the agent deleted, and one holding commits of its own, are left
// alone.
func (service *DrudgerService) dropEmptyBranch(repository repositoryWorktree, landing task.Landing, head string) error {
	hasBranch, err := service.gitOps.BranchExists(repository.Worktree, landing.Branch)
	if err != nil {
		return err
	}
	if !hasBranch {
		return nil
	}

	hasCommits, err := git.BranchHasCommits(service.gitOps, repository.Worktree, landing.Base, landing.Branch)
	if err != nil {
		return err
	}
	if hasCommits {
		return nil
	}

	current, err := service.gitOps.CurrentBranch(repository.Worktree)
	if err != nil {
		return err
	}
	if current == landing.Branch {
		if err := service.gitOps.CheckoutDetached(repository.Worktree, head); err != nil {
			return err
		}
	}

	if err := service.gitOps.DeleteBranch(repository.Worktree, landing.Branch); err != nil {
		return err
	}
	service.progress.Report(EmptyBranchDropped{Repository: repository.Name, Branch: landing.Branch})
	return nil
}
