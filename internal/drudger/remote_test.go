package drudger

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const testUnparsableURL = "/srv/git/api.git"

// testTemplatePath is the one place fakeRemote keeps a pull request template.
const testTemplatePath = ".github/pull_request_template.md"

// fakeRemote accepts every remote URL but testUnparsableURL, and opens every
// pull request it is asked to.
type fakeRemote struct {
	readyErr error
	// openErrs fail opening a pull request, keyed by its title.
	openErrs map[string]error
	opened   []remote.PullRequestDto
}

func (fake *fakeRemote) Provider() remote.Provider {
	return remote.ProviderGitHub
}

func (fake *fakeRemote) CheckReady() error {
	return fake.readyErr
}

func (fake *fakeRemote) TemplatePaths() []string {
	return []string{testTemplatePath}
}

func (fake *fakeRemote) ParseRepository(url string) (remote.Repository, error) {
	if url == testUnparsableURL {
		return remote.Repository{}, errors.New("not a GitHub repository URL")
	}
	return remote.Repository{Host: "github.com", Owner: "owner", Name: "api"}, nil
}

func (fake *fakeRemote) OpenPullRequest(dto remote.PullRequestDto) (string, error) {
	if err := fake.openErrs[dto.Title]; err != nil {
		return "", err
	}
	fake.opened = append(fake.opened, dto)
	return testPullRequestURL(len(fake.opened)), nil
}

// testPullRequestURL is the URL of the pull request a fake remote opens as the
// one numbered number.
func testPullRequestURL(number int) string {
	return fmt.Sprintf("https://github.com/owner/api/pull/%d", number)
}

func TestDrudgerService_PreRunCheck(t *testing.T) {
	commands := []struct {
		name     string
		status   task.TaskStatus
		isDryRun bool
		start    func(service *testService, taskID task.TaskID, isDryRun bool) error
	}{
		{
			name:   "run",
			status: task.StatusTodo,
			start: func(service *testService, taskID task.TaskID, isDryRun bool) error {
				return service.RunTask(testProjectSlug, taskID, isDryRun)
			},
		},
		{
			name:     "dry run",
			status:   task.StatusTodo,
			isDryRun: true,
			start: func(service *testService, taskID task.TaskID, isDryRun bool) error {
				return service.RunTask(testProjectSlug, taskID, isDryRun)
			},
		},
		{
			name:   "rerun",
			status: task.StatusFuckedUp,
			start: func(service *testService, taskID task.TaskID, isDryRun bool) error {
				return service.RerunTask(testProjectSlug, taskID, isDryRun)
			},
		},
		{
			name:     "dry rerun",
			status:   task.StatusFuckedUp,
			isDryRun: true,
			start: func(service *testService, taskID task.TaskID, isDryRun bool) error {
				return service.RerunTask(testProjectSlug, taskID, isDryRun)
			},
		},
	}
	checks := []struct {
		name string
		// remote is nil for a project with pull requests off.
		remote *fakeRemote
		setup  func(git *fakeGit)
		// wantErrText lists fragments the error must carry. A check with none
		// lets the run through.
		wantErrText []string
	}{
		{name: "pull requests off", setup: func(git *fakeGit) { git.hasNoRemote = true }},
		{name: "ready project", remote: &fakeRemote{}},
		{
			name:        "provider not ready",
			remote:      &fakeRemote{readyErr: errors.New("gh is not logged in to GitHub, run gh auth login")},
			wantErrText: []string{"gh auth login"},
		},
		{
			name:        "repository with no origin",
			remote:      &fakeRemote{},
			setup:       func(git *fakeGit) { git.hasNoRemote = true },
			wantErrText: []string{"no origin remote", remote.PullRequestsEnabledKey},
		},
		{
			name:        "origin the provider cannot read",
			remote:      &fakeRemote{},
			setup:       func(git *fakeGit) { git.remoteURL = testUnparsableURL },
			wantErrText: []string{"not on github", "not a GitHub repository URL", remote.PullRequestsEnabledKey},
		},
	}

	for _, command := range commands {
		for _, check := range checks {
			t.Run(command.name+"/"+check.name, func(t *testing.T) {
				projectDir := setupProjectDir(t)
				taskToRun := todoTask()
				taskToRun.Status = command.status
				runner := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith(testSandbox)}}
				service := newTestServiceWith(testSettings(), runner, taskToRun)
				if check.remote != nil {
					service.remote = check.remote
				}
				if check.setup != nil {
					check.setup(service.git)
				}

				err := command.start(service, taskToRun.ID, command.isDryRun)

				if len(check.wantErrText) == 0 {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					return
				}
				if err == nil {
					t.Fatal("expected the run to be refused")
				}
				for _, fragment := range check.wantErrText {
					if !strings.Contains(err.Error(), fragment) {
						t.Errorf("error = %q, want it to name %q", err, fragment)
					}
				}
				if len(service.drudgers.drudgers) != 0 {
					t.Errorf("expected no Drudger to be claimed, got %v", service.drudgers.drudgers)
				}
				if len(runner.calls) != 0 || len(runner.startedSubcommands()) != 0 {
					t.Errorf("expected nothing to run, got %v", runner.subcommands())
				}
			})
		}
	}
}

func TestDrudgerService_RunTask_DryRunNamesThePullRequestProvider(t *testing.T) {
	tests := []struct {
		name   string
		remote remote.Remote
		want   remote.Provider
	}{
		{name: "pull requests off"},
		{name: "pull requests on", remote: &fakeRemote{}, want: remote.ProviderGitHub},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupProjectDir(t)
			taskToRun := todoTask()
			service := newTestService(taskToRun)
			service.remote = test.remote

			if err := service.RunTask(testProjectSlug, taskToRun.ID, true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if described := singleEvent[RunDescribed](t, service.progress); described.PullRequestProvider != test.want {
				t.Errorf("PullRequestProvider = %q, want %q", described.PullRequestProvider, test.want)
			}
		})
	}
}
