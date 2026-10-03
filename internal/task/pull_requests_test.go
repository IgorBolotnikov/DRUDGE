package task

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const (
	apiPullRequest  = "https://github.com/acme/api/pull/12"
	uiPullRequest   = "https://github.com/acme/ui/pull/7"
	docsPullRequest = "http://git.acme.internal/acme/docs/pull/3"
)

func TestTaskService_EditTask_PullRequests(t *testing.T) {
	cases := []struct {
		name               string
		storedPullRequests []string
		changes            EditTaskDto
		wantPullRequests   []string
		// wantErrText are fragments a refusal must carry. A case without them
		// expects the edit to go through.
		wantErrText []string
	}{
		{
			name:             "setting a list",
			changes:          EditTaskDto{PullRequests: &[]string{apiPullRequest, docsPullRequest}},
			wantPullRequests: []string{apiPullRequest, docsPullRequest},
		},
		{
			name:               "replacing a list",
			storedPullRequests: []string{apiPullRequest},
			changes:            EditTaskDto{PullRequests: &[]string{uiPullRequest}},
			wantPullRequests:   []string{uiPullRequest},
		},
		{
			name:               "replacing a list with repeats",
			storedPullRequests: []string{apiPullRequest},
			changes:            EditTaskDto{PullRequests: &[]string{uiPullRequest, uiPullRequest}},
			wantPullRequests:   []string{uiPullRequest},
		},
		{
			name:               "clearing a list",
			storedPullRequests: []string{apiPullRequest},
			changes:            EditTaskDto{PullRequests: &[]string{}},
			wantPullRequests:   nil,
		},
		{
			name:             "adding to an empty list",
			changes:          EditTaskDto{AddPullRequests: &[]string{apiPullRequest}},
			wantPullRequests: []string{apiPullRequest},
		},
		{
			name:               "adding a URL already on the list",
			storedPullRequests: []string{apiPullRequest},
			changes:            EditTaskDto{AddPullRequests: &[]string{apiPullRequest, uiPullRequest}},
			wantPullRequests:   []string{apiPullRequest, uiPullRequest},
		},
		{
			name:        "adding nothing",
			changes:     EditTaskDto{AddPullRequests: &[]string{}},
			wantErrText: []string{"name at least one pull request to add"},
		},
		{
			name:               "removing one of several",
			storedPullRequests: []string{apiPullRequest, uiPullRequest, docsPullRequest},
			changes:            EditTaskDto{RemovePullRequests: &[]string{uiPullRequest}},
			wantPullRequests:   []string{apiPullRequest, docsPullRequest},
		},
		{
			name:               "removing the last one",
			storedPullRequests: []string{apiPullRequest},
			changes:            EditTaskDto{RemovePullRequests: &[]string{apiPullRequest}},
			wantPullRequests:   nil,
		},
		{
			name:               "removing one that is not there",
			storedPullRequests: []string{apiPullRequest},
			changes:            EditTaskDto{RemovePullRequests: &[]string{uiPullRequest}},
			wantErrText:        []string{"task 006684e3 has no pull request " + uiPullRequest},
		},
		{
			name:        "removing nothing",
			changes:     EditTaskDto{RemovePullRequests: &[]string{}},
			wantErrText: []string{"name at least one pull request to remove"},
		},
		{
			name: "replacing the list and adding",
			changes: EditTaskDto{
				PullRequests:    &[]string{apiPullRequest},
				AddPullRequests: &[]string{uiPullRequest},
			},
			wantErrText: []string{"the pull request list", "the pull requests to add"},
		},
		{
			name: "adding and removing",
			changes: EditTaskDto{
				AddPullRequests:    &[]string{apiPullRequest},
				RemovePullRequests: &[]string{uiPullRequest},
			},
			wantErrText: []string{"the pull requests to add", "the pull requests to remove"},
		},
		{
			name:        "a URL with no scheme",
			changes:     EditTaskDto{PullRequests: &[]string{"github.com/acme/api/pull/12"}},
			wantErrText: []string{`"github.com/acme/api/pull/12"`, "absolute http or https URL"},
		},
		{
			name:        "a URL of another scheme",
			changes:     EditTaskDto{AddPullRequests: &[]string{"ssh://git@github.com/acme/api"}},
			wantErrText: []string{`"ssh://git@github.com/acme/api"`, "absolute http or https URL"},
		},
		{
			name:        "a URL with no host",
			changes:     EditTaskDto{AddPullRequests: &[]string{"https:///acme/api/pull/12"}},
			wantErrText: []string{"absolute http or https URL"},
		},
		{
			name:        "text that does not parse",
			changes:     EditTaskDto{AddPullRequests: &[]string{"https://github.com/%zz"}},
			wantErrText: []string{"is not a URL"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stored := editableTask()
			stored.PullRequests = testCase.storedPullRequests
			repo := &fakeTaskRepo{tasks: []*Task{stored}}
			service := NewTaskService(repo, &fakeProgress{}, StatusDraft)

			edited, err := service.EditTask(testProjectSlug, editableTaskID, testCase.changes, &fakeSessionGuard{})

			if testCase.wantErrText != nil {
				if err == nil {
					t.Fatal("expected the edit to be refused")
				}
				for _, want := range testCase.wantErrText {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("expected the error to carry %q, got %q", want, err)
					}
				}
				if repo.writes != 0 {
					t.Errorf("expected a refused edit to write nothing, got %d writes", repo.writes)
				}
				if !slices.Equal(stored.PullRequests, testCase.storedPullRequests) {
					t.Errorf("expected the pull requests to stay %v, got %v", testCase.storedPullRequests, stored.PullRequests)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(edited.PullRequests, testCase.wantPullRequests) {
				t.Errorf("expected the pull requests %#v, got %#v", testCase.wantPullRequests, edited.PullRequests)
			}
			if repo.writes != 1 {
				t.Errorf("expected one write, got %d", repo.writes)
			}
		})
	}
}

func TestTaskService_EditTask_RefusesPullRequestsWhileASessionWorks(t *testing.T) {
	refusal := errors.New("an agent is working on task 006684e3")
	stored := editableTask()
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, &fakeProgress{}, StatusDraft)

	_, err := service.EditTask(testProjectSlug, editableTaskID, EditTaskDto{AddPullRequests: &[]string{apiPullRequest}}, &fakeSessionGuard{refusal: refusal})

	if !errors.Is(err, refusal) {
		t.Fatalf("expected the Session guard to refuse the edit, got %v", err)
	}
	if repo.writes != 0 || stored.PullRequests != nil {
		t.Errorf("expected a refused edit to change nothing, got %d writes and %v", repo.writes, stored.PullRequests)
	}
}

func TestTaskService_CreateTask_PullRequests(t *testing.T) {
	cases := []struct {
		name             string
		pullRequests     []string
		wantPullRequests []string
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects the task to be created.
		wantErrText string
	}{
		{
			name: "no pull requests",
		},
		{
			name:             "several pull requests",
			pullRequests:     []string{apiPullRequest, docsPullRequest},
			wantPullRequests: []string{apiPullRequest, docsPullRequest},
		},
		{
			name:             "repeats collapsing",
			pullRequests:     []string{apiPullRequest, apiPullRequest},
			wantPullRequests: []string{apiPullRequest},
		},
		{
			name:         "a URL with no scheme",
			pullRequests: []string{"acme/api#12"},
			wantErrText:  `"acme/api#12"`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{}
			service := NewTaskService(repo, &fakeProgress{}, StatusDraft)

			created, err := service.CreateTask(CreateTaskDto{
				Title:        "Wire the service",
				ProjectSlug:  testProjectSlug,
				PullRequests: testCase.pullRequests,
			})

			if testCase.wantErrText != "" {
				if err == nil {
					t.Fatal("expected the task to be refused")
				}
				if !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Errorf("expected the error to carry %q, got %q", testCase.wantErrText, err)
				}
				if len(repo.tasks) != 0 {
					t.Errorf("expected a refused task to be left unwritten, got %d tasks", len(repo.tasks))
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(created.PullRequests, testCase.wantPullRequests) {
				t.Errorf("expected the pull requests %#v, got %#v", testCase.wantPullRequests, created.PullRequests)
			}
		})
	}
}

func TestTask_RecordPullRequest(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		recorded string
		want     []string
	}{
		{name: "a task with no pull requests", recorded: apiPullRequest, want: []string{apiPullRequest}},
		{name: "a task with another pull request", existing: []string{apiPullRequest}, recorded: uiPullRequest, want: []string{apiPullRequest, uiPullRequest}},
		{name: "a URL the task already carries", existing: []string{apiPullRequest}, recorded: apiPullRequest, want: []string{apiPullRequest}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			taskToRecord := &Task{PullRequests: slices.Clone(testCase.existing)}

			taskToRecord.RecordPullRequest(testCase.recorded)

			if !slices.Equal(taskToRecord.PullRequests, testCase.want) {
				t.Errorf("expected the pull requests %v, got %v", testCase.want, taskToRecord.PullRequests)
			}
		})
	}
}
