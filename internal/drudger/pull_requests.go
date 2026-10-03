package drudger

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
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
	DescriptionReadStep   PullRequestStep = "read description"
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

// PullRequestFailed reports a pull request close-out or a retry could not
// open. The description stays in the run directory, except when the move step
// failed and it stays in the worktree. A failed removal comes after the pull
// request is open.
type PullRequestFailed struct {
	TaskID     task.TaskID
	Repository string
	Step       PullRequestStep
	Err        error
}

// PullRequestRetryStarted reports a retry opening the pull requests a task
// has descriptions left for.
type PullRequestRetryStarted struct {
	Task *task.Task
}

// NoPullRequestLeft reports a repository the task has a landing in and whose
// pull request description is gone from the run directory. Its pull request
// is open, or the agent never wrote a description.
type NoPullRequestLeft struct {
	Repository string
	Branch     string
}

// PullRequestWithoutLanding reports a pull request description left in the
// run directory of a task with no landing in its repository. A retry leaves
// it alone.
type PullRequestWithoutLanding struct {
	TaskID     task.TaskID
	Repository string
}

// PullRequestsRetried reports how a retry went. Tried counts the pull requests
// it had a description for.
type PullRequestsRetried struct {
	Task   *task.Task
	Tried  int
	Opened int
}

// RetryPullRequests opens the pull requests close-out left behind for a task.
// Every description left in the run directory goes through the steps of
// close-out, with the branch pushed from the repository in the project
// directory. The slot the task ran in may hold another task by now.
//
// It refuses a project with pull requests off or failing the pre-run check, a
// task whose Session is still working, and a task another command holds. A
// repository whose pull request fails is reported and the rest are still
// tried.
func (service *DrudgerService) RetryPullRequests(projectSlug string, requestedID task.TaskID) error {
	found, err := service.tasks.GetTask(projectSlug, requestedID)
	if err != nil {
		return err
	}
	if service.remote == nil {
		return fmt.Errorf("pull requests are off, turn on %s to open the pull requests of task %s", remote.PullRequestsEnabledKey, found.ID)
	}

	layout, err := service.layout()
	if err != nil {
		return err
	}
	if err := service.checkRemote(layout); err != nil {
		return err
	}
	repositories, err := service.resolveRepositories(layout)
	if err != nil {
		return err
	}

	var retried PullRequestsRetried
	isStored, err := service.tasks.TryUpdateTask(projectSlug, found.ID, func(onDisk *task.Task) error {
		if err := service.RefuseWhileWorking(projectSlug, onDisk); err != nil {
			return err
		}
		left, err := service.runs.ListPullRequests(onDisk.ID)
		if err != nil {
			return err
		}

		service.progress.Report(PullRequestRetryStarted{Task: onDisk})
		retried = service.retryPullRequests(onDisk, repositories, left)
		if retried.Opened == 0 {
			return task.ErrTaskUnchanged
		}
		return nil
	})
	if err != nil {
		if retried.Opened > 0 {
			return fmt.Errorf("opened %d pull requests of task %s, but they could not be recorded on the task: %w", retried.Opened, found.ID, err)
		}
		return err
	}
	if !isStored {
		return fmt.Errorf("another drudge command is working on task %s, wait for it to finish and run this again", found.ID)
	}

	service.progress.Report(retried)
	return nil
}

// retryPullRequests opens a pull request for every landing of a task with a
// description left, and reports the landings and the descriptions that do not
// pair up.
func (service *DrudgerService) retryPullRequests(retried *task.Task, repositories []projectRepository, left []string) PullRequestsRetried {
	result := PullRequestsRetried{Task: retried}

	for _, name := range slices.Sorted(maps.Keys(retried.Landings)) {
		landing := retried.Landings[name]
		if !slices.Contains(left, name) {
			service.progress.Report(NoPullRequestLeft{Repository: name, Branch: landing.Branch})
			continue
		}

		result.Tried++
		index := slices.IndexFunc(repositories, func(repository projectRepository) bool { return repository.Name == name })
		if index < 0 {
			err := fmt.Errorf("project %s records no repository %s to push from", service.settings.ProjectSlug, name)
			service.reportPullRequestFailed(retried.ID, name, BranchPushStep, err)
			continue
		}
		repository := repositories[index]
		if service.openPullRequest(retried, repository, repository.Dir, landing) {
			result.Opened++
		}
	}

	for _, name := range left {
		if _, hasLanding := retried.Landings[name]; !hasLanding {
			service.progress.Report(PullRequestWithoutLanding{TaskID: retried.ID, Repository: name})
		}
	}
	return result
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
			service.reportPullRequestFailed(finished.ID, repository.Name, DescriptionMoveStep, err)
			continue
		}
		if !hasDescription {
			service.progress.Report(PullRequestDescriptionMissing{Repository: repository.Name, Branch: landing.Branch})
			continue
		}

		service.openPullRequest(finished, repository.projectRepository, repository.Worktree, landing)
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

// openPullRequest pushes the branch of one repository from gitDir and opens a
// pull request from it into the default branch, with the description in the
// run directory. The URL goes on the task and the description is deleted. It
// reports whether the pull request was opened.
func (service *DrudgerService) openPullRequest(finished *task.Task, repository projectRepository, gitDir string, landing task.Landing) bool {
	description, isPresent, err := service.runs.ReadPullRequest(finished.ID, repository.Name)
	if err == nil && !isPresent {
		err = fmt.Errorf("the run directory of task %s holds no pull request description of repository %s", finished.ID, repository.Name)
	}
	if err != nil {
		service.reportPullRequestFailed(finished.ID, repository.Name, DescriptionReadStep, err)
		return false
	}
	title, body := parsePullRequest(description, finished.Title)

	remoteURL, err := service.gitOps.RemoteURL(gitDir, git.OriginRemote)
	if err != nil {
		service.reportPullRequestFailed(finished.ID, repository.Name, PullRequestOpenStep, err)
		return false
	}
	hosted, err := service.remote.ParseRepository(remoteURL)
	if err != nil {
		service.reportPullRequestFailed(finished.ID, repository.Name, PullRequestOpenStep, err)
		return false
	}

	service.progress.Report(BranchPushStarted{Repository: repository.Name, Branch: landing.Branch, Remote: git.OriginRemote})
	if err := service.gitOps.Push(gitDir, git.OriginRemote, landing.Branch); err != nil {
		service.reportPullRequestFailed(finished.ID, repository.Name, BranchPushStep, err)
		return false
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
		service.reportPullRequestFailed(finished.ID, repository.Name, PullRequestOpenStep, err)
		return false
	}

	finished.RecordPullRequest(pullRequestURL)
	service.progress.Report(PullRequestOpened{Repository: repository.Name, URL: pullRequestURL})

	if err := service.runs.RemovePullRequest(finished.ID, repository.Name); err != nil {
		service.reportPullRequestFailed(finished.ID, repository.Name, DescriptionRemoveStep, err)
	}
	return true
}

func (service *DrudgerService) reportPullRequestFailed(taskID task.TaskID, repository string, step PullRequestStep, err error) {
	service.progress.Report(PullRequestFailed{TaskID: taskID, Repository: repository, Step: step, Err: err})
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
