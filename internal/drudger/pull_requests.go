package drudger

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// titleHeadingPrefix is the markdown heading marker an agent may put before
// the title of a pull request.
const titleHeadingPrefix = "# "

// PullRequestStep names the step of opening a pull request that failed.
type PullRequestStep string

const (
	DescriptionMoveStep   PullRequestStep = "move description"
	BranchPushStep        PullRequestStep = "push"
	PullRequestOpenStep   PullRequestStep = "open"
	DescriptionRemoveStep PullRequestStep = "remove description"
)

// BranchPushStarted reports close-out pushing the branch of one repository.
type BranchPushStarted struct {
	Repository string
	Branch     string
	Remote     string
}

// PullRequestOpened reports a pull request close-out opened for the work of
// one repository.
type PullRequestOpened struct {
	Repository string
	URL        string
}

// PullRequestDescriptionMissing reports a repository the agent committed to
// without writing a pull request description. Its branch is not pushed.
type PullRequestDescriptionMissing struct {
	Repository string
	Branch     string
}

// PullRequestFailed reports a pull request close-out could not open. The
// description stays in the run directory, except when the move step failed
// and it stays in the worktree. A failed removal comes after the pull request
// is open.
type PullRequestFailed struct {
	TaskID     task.TaskID
	Repository string
	Step       PullRequestStep
	Err        error
}

// openPullRequests opens a pull request for every repository of a workspace
// the task still has a landing in. A repository whose pull request fails is
// reported and the rest still get theirs.
func (service *DrudgerService) openPullRequests(space slotWorkspace, finished *task.Task) {
	for _, repository := range space.Repositories {
		landing, hasLanding := finished.Landings[repository.Name]
		if !hasLanding {
			continue
		}

		hasDescription, err := service.moveDescription(finished.ID, repository)
		if err != nil {
			service.reportPullRequestFailed(finished.ID, repository, DescriptionMoveStep, err)
			continue
		}
		if !hasDescription {
			service.progress.Report(PullRequestDescriptionMissing{Repository: repository.Name, Branch: landing.Branch})
			continue
		}

		service.openPullRequest(finished, repository, landing)
	}
}

// moveDescription moves the pull request description an agent wrote in a
// worktree into the run directory. It reports false for a worktree with no
// description.
func (service *DrudgerService) moveDescription(taskID task.TaskID, repository repositoryWorktree) (bool, error) {
	path := filepath.Join(repository.Worktree, pullRequestFilePath)
	isPresent, err := common.Exists(path)
	if err != nil || !isPresent {
		return false, err
	}

	description, err := common.ReadFile(path)
	if err != nil {
		return false, err
	}
	if err := service.runs.WritePullRequest(taskID, repository.Name, description); err != nil {
		return false, err
	}
	if err := common.RemoveAll(path); err != nil {
		return false, err
	}
	return true, nil
}

// openPullRequest pushes the branch of one repository and opens a pull request
// from it into the default branch, with the description in the run directory.
// The URL goes on the task and the description is deleted.
func (service *DrudgerService) openPullRequest(finished *task.Task, repository repositoryWorktree, landing task.Landing) {
	description, isPresent, err := service.runs.ReadPullRequest(finished.ID, repository.Name)
	if err == nil && !isPresent {
		err = fmt.Errorf("the run directory of task %s holds no pull request description of repository %s", finished.ID, repository.Name)
	}
	if err != nil {
		service.reportPullRequestFailed(finished.ID, repository, DescriptionMoveStep, err)
		return
	}
	title, body := parsePullRequest(description, finished.Title)

	remoteURL, err := service.gitOps.RemoteURL(repository.Worktree, git.OriginRemote)
	if err != nil {
		service.reportPullRequestFailed(finished.ID, repository, PullRequestOpenStep, err)
		return
	}
	hosted, err := service.remote.ParseRepository(remoteURL)
	if err != nil {
		service.reportPullRequestFailed(finished.ID, repository, PullRequestOpenStep, err)
		return
	}

	service.progress.Report(BranchPushStarted{Repository: repository.Name, Branch: landing.Branch, Remote: git.OriginRemote})
	if err := service.gitOps.Push(repository.Worktree, git.OriginRemote, landing.Branch); err != nil {
		service.reportPullRequestFailed(finished.ID, repository, BranchPushStep, err)
		return
	}

	pullRequestURL, err := service.remote.OpenPullRequest(remote.PullRequestDto{
		Repository: hosted,
		Base:       repository.DefaultBranch,
		Head:       landing.Branch,
		Title:      title,
		Body:       body,
		IsDraft:    service.settings.PullRequests.IsDraft,
	})
	if err != nil {
		// A provider that is not ready says what the user has to set up.
		if readyErr := service.remote.CheckReady(); readyErr != nil {
			err = readyErr
		}
		service.reportPullRequestFailed(finished.ID, repository, PullRequestOpenStep, err)
		return
	}

	finished.RecordPullRequest(pullRequestURL)
	service.progress.Report(PullRequestOpened{Repository: repository.Name, URL: pullRequestURL})

	if err := service.runs.RemovePullRequest(finished.ID, repository.Name); err != nil {
		service.reportPullRequestFailed(finished.ID, repository, DescriptionRemoveStep, err)
	}
}

func (service *DrudgerService) reportPullRequestFailed(taskID task.TaskID, repository repositoryWorktree, step PullRequestStep, err error) {
	service.progress.Report(PullRequestFailed{TaskID: taskID, Repository: repository.Name, Step: step, Err: err})
}

// parsePullRequest reads the title and the body of a pull request description.
// The first line is the title, without a leading heading marker. The rest,
// trimmed, is the body. A blank title falls back to fallbackTitle.
func parsePullRequest(description string, fallbackTitle string) (title string, body string) {
	firstLine, rest, _ := strings.Cut(description, "\n")
	title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(firstLine), titleHeadingPrefix))
	if title == "" {
		title = fallbackTitle
	}
	return title, strings.TrimSpace(rest)
}
