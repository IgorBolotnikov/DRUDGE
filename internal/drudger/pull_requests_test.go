package drudger

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// testTaskTitle is the title of the task todoTask builds.
const testTaskTitle = "Fix login"

// pullRequestFailure is one pull request close-out reports as failed.
type pullRequestFailure struct {
	repository string
	step       PullRequestStep
	// err is the error the failure carries. Nil accepts any error.
	err error
}

func TestDrudgerService_SessionStatus_OpensPullRequests(t *testing.T) {
	pushErr := errors.New("remote rejected the push")
	openErr := errors.New("a pull request already exists")
	readyErr := errors.New("gh is not logged in to GitHub, run gh auth login")
	testRepository := remote.Repository{Host: "github.com", Owner: "owner", Name: "api"}

	// openedFrom is the pull request close-out opens from a description with
	// this title and body.
	openedFrom := func(title string, body string) remote.PullRequestDto {
		return remote.PullRequestDto{Repository: testRepository, Base: testDefaultBranch, Head: testTaskBranch, Title: title, Body: body}
	}

	cases := []struct {
		name string
		// repositories are the repositories of the project. The agent committed
		// in every one of them.
		repositories []string
		// isDisabled turns pull requests off.
		isDisabled bool
		isDraft    bool
		// result is the last event of the stream. Empty means a clean result.
		result string
		// descriptions are what the agent wrote as the pull request
		// description of each repository, keyed by repository name.
		descriptions map[string]string
		// pushErrs fail the push of a repository, keyed by repository name.
		pushErrs map[string]error
		// openErrs fail opening a pull request, keyed by its title.
		openErrs map[string]error
		readyErr error

		wantStatus       task.TaskStatus
		wantOpened       []remote.PullRequestDto
		wantPullRequests []string
		wantPushedIn     []string
		wantMissingIn    []string
		wantFailures     []pullRequestFailure
		// wantKeptIn names the repositories whose description stays in the run
		// directory.
		wantKeptIn []string
		// wantLeftIn names the repositories whose description stays in the
		// worktree.
		wantLeftIn []string
	}{
		{
			name:         "pull requests off",
			repositories: []string{"api"},
			isDisabled:   true,
			descriptions: map[string]string{"api": "Add retry\n\nRetries uploads."},
			wantStatus:   task.StatusUnmerged,
			wantLeftIn:   []string{"api"},
		},
		{
			name:         "a Session that fucked up",
			repositories: []string{"api"},
			result:       erroredResultEvent,
			descriptions: map[string]string{"api": "Add retry\n\nRetries uploads."},
			wantStatus:   task.StatusFuckedUp,
			wantLeftIn:   []string{"api"},
		},
		{
			name:             "one repository with a description and one without",
			repositories:     []string{"api", "ui"},
			descriptions:     map[string]string{"api": "Add retry\n\nRetries uploads."},
			wantStatus:       task.StatusUnmerged,
			wantOpened:       []remote.PullRequestDto{openedFrom("Add retry", "Retries uploads.")},
			wantPullRequests: []string{testPullRequestURL(1)},
			wantPushedIn:     []string{"api"},
			wantMissingIn:    []string{"ui"},
		},
		{
			name:             "a draft",
			repositories:     []string{"api"},
			isDraft:          true,
			descriptions:     map[string]string{"api": "Add retry\n\nRetries uploads."},
			wantStatus:       task.StatusUnmerged,
			wantOpened:       []remote.PullRequestDto{{Repository: testRepository, Base: testDefaultBranch, Head: testTaskBranch, Title: "Add retry", Body: "Retries uploads.", IsDraft: true}},
			wantPullRequests: []string{testPullRequestURL(1)},
			wantPushedIn:     []string{"api"},
		},
		{
			name:             "a push that fails",
			repositories:     []string{"api", "ui"},
			descriptions:     map[string]string{"api": "Add retry", "ui": "Show retries"},
			pushErrs:         map[string]error{"api": pushErr},
			wantStatus:       task.StatusUnmerged,
			wantOpened:       []remote.PullRequestDto{openedFrom("Show retries", "")},
			wantPullRequests: []string{testPullRequestURL(1)},
			wantPushedIn:     []string{"ui"},
			wantFailures:     []pullRequestFailure{{repository: "api", step: BranchPushStep, err: pushErr}},
			wantKeptIn:       []string{"api"},
		},
		{
			name:             "a pull request that fails",
			repositories:     []string{"api", "ui"},
			descriptions:     map[string]string{"api": "Add retry", "ui": "Show retries"},
			openErrs:         map[string]error{"Add retry": openErr},
			wantStatus:       task.StatusUnmerged,
			wantOpened:       []remote.PullRequestDto{openedFrom("Show retries", "")},
			wantPullRequests: []string{testPullRequestURL(1)},
			wantPushedIn:     []string{"api", "ui"},
			wantFailures:     []pullRequestFailure{{repository: "api", step: PullRequestOpenStep, err: openErr}},
			wantKeptIn:       []string{"api"},
		},
		{
			name:         "a pull request that fails on a provider that is not ready",
			repositories: []string{"api"},
			descriptions: map[string]string{"api": "Add retry"},
			openErrs:     map[string]error{"Add retry": openErr},
			readyErr:     readyErr,
			wantStatus:   task.StatusUnmerged,
			wantPushedIn: []string{"api"},
			wantFailures: []pullRequestFailure{{repository: "api", step: PullRequestOpenStep, err: readyErr}},
			wantKeptIn:   []string{"api"},
		},
		{
			name:             "a title written as a heading",
			repositories:     []string{"api"},
			descriptions:     map[string]string{"api": "# Add retry\n\n## What\n\nRetries uploads.\n"},
			wantStatus:       task.StatusUnmerged,
			wantOpened:       []remote.PullRequestDto{openedFrom("Add retry", "## What\n\nRetries uploads.")},
			wantPullRequests: []string{testPullRequestURL(1)},
			wantPushedIn:     []string{"api"},
		},
		{
			name:             "a blank first line",
			repositories:     []string{"api"},
			descriptions:     map[string]string{"api": "\nRetries uploads."},
			wantStatus:       task.StatusUnmerged,
			wantOpened:       []remote.PullRequestDto{openedFrom(testTaskTitle, "Retries uploads.")},
			wantPullRequests: []string{testPullRequestURL(1)},
			wantPushedIn:     []string{"api"},
		},
		{
			name:             "an empty description",
			repositories:     []string{"api"},
			descriptions:     map[string]string{"api": ""},
			wantStatus:       task.StatusUnmerged,
			wantOpened:       []remote.PullRequestDto{openedFrom(testTaskTitle, "")},
			wantPullRequests: []string{testPullRequestURL(1)},
			wantPushedIn:     []string{"api"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			tracked := handedOverTask(testCase.repositories...)

			settings := settingsWith(testCase.repositories...)
			settings.PullRequests.IsDraft = testCase.isDraft
			pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
			service := newTestServiceWithPool(settings, &fakeCommandRunner{}, pool, tracked)
			pullRequestRemote := &fakeRemote{readyErr: testCase.readyErr, openErrs: testCase.openErrs}
			if !testCase.isDisabled {
				service.remote = pullRequestRemote
			}

			result := testCase.result
			if result == "" {
				result = resultEvent
			}
			service.runs.writeStream(tracked.ID, initEvent, result)
			service.runs.writeExit(tracked.ID, "0\n")

			worktrees := worktreesOf(projectDir, testCase.repositories)
			makeWorktrees(t, worktrees)
			for _, repository := range testCase.repositories {
				service.git.leaveOn(worktrees[repository], testTaskBranch, testHeadSHA)
			}
			service.git.commitsOn(testHeadSHA, 2)
			for repository, description := range testCase.descriptions {
				writeTestFile(t, filepath.Join(worktrees[repository], pullRequestFilePath), description)
			}
			service.git.pushErrs = map[string]error{}
			for repository, err := range testCase.pushErrs {
				service.git.pushErrs[worktrees[repository]] = err
			}

			session, err := service.SessionStatus(testProjectSlug, tracked.ID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if session.Task.Status != testCase.wantStatus {
				t.Errorf("expected status %q, got %q", testCase.wantStatus, session.Task.Status)
			}
			if !slices.EqualFunc(pullRequestRemote.opened, testCase.wantOpened, func(got, want remote.PullRequestDto) bool { return got == want }) {
				t.Errorf("expected the pull requests %+v to be opened, got %+v", testCase.wantOpened, pullRequestRemote.opened)
			}
			if !slices.Equal(session.Task.PullRequests, testCase.wantPullRequests) {
				t.Errorf("expected the task to carry %v, got %v", testCase.wantPullRequests, session.Task.PullRequests)
			}

			wantPushed := make([]branchRef, 0, len(testCase.wantPushedIn))
			for _, repository := range testCase.wantPushedIn {
				wantPushed = append(wantPushed, branchRef{dir: worktrees[repository], branch: testTaskBranch})
			}
			if !slices.Equal(service.git.pushed, wantPushed) {
				t.Errorf("expected the pushes %v, got %v", wantPushed, service.git.pushed)
			}

			var missingIn []string
			for _, missing := range reportedEvents[PullRequestDescriptionMissing](service.progress) {
				missingIn = append(missingIn, missing.Repository)
			}
			if !slices.Equal(missingIn, testCase.wantMissingIn) {
				t.Errorf("expected a missing description in %v, got %v", testCase.wantMissingIn, missingIn)
			}

			failures := reportedEvents[PullRequestFailed](service.progress)
			if len(failures) != len(testCase.wantFailures) {
				t.Fatalf("expected the failures %+v, got %+v", testCase.wantFailures, failures)
			}
			for index, want := range testCase.wantFailures {
				got := failures[index]
				if got.TaskID != tracked.ID || got.Repository != want.repository || got.Step != want.step || got.Err == nil {
					t.Errorf("expected a failure to %s in %s, got %+v", want.step, want.repository, got)
				}
				if want.err != nil && !errors.Is(got.Err, want.err) {
					t.Errorf("expected the failure in %s to carry %v, got %v", want.repository, want.err, got.Err)
				}
			}

			for _, repository := range testCase.repositories {
				_, isKept := service.runs.file(tracked.ID, pullRequestFileName(repository))
				if wantKept := slices.Contains(testCase.wantKeptIn, repository); isKept != wantKept {
					t.Errorf("description of %s kept in the run directory = %t, want %t", repository, isKept, wantKept)
				}

				isLeft, err := common.Exists(filepath.Join(worktrees[repository], pullRequestFilePath))
				if err != nil {
					t.Fatalf("could not check the description in %s: %v", repository, err)
				}
				if wantLeft := slices.Contains(testCase.wantLeftIn, repository); isLeft != wantLeft {
					t.Errorf("description of %s left in the worktree = %t, want %t", repository, isLeft, wantLeft)
				}
			}
		})
	}
}
