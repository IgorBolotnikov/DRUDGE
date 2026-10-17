package task

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const (
	apiRepository  = "api"
	uiRepository   = "ui"
	docsRepository = "docs"
	selfRepository = "."
)

// multiRepositoryProject records the repositories of a project directory that
// holds several of them.
var multiRepositoryProject = []string{apiRepository, uiRepository, docsRepository}

func TestTaskService_EditTask_Repositories(t *testing.T) {
	cases := []struct {
		name                string
		projectRepositories []string
		storedRepositories  []string
		changes             EditTaskDto
		wantRepositories    []string
		// wantErrText are fragments a refusal must carry. A case without them
		// expects the edit to go through.
		wantErrText []string
	}{
		{
			name:             "setting a list",
			changes:          EditTaskDto{Repositories: &[]string{apiRepository, docsRepository}},
			wantRepositories: []string{apiRepository, docsRepository},
		},
		{
			name:               "replacing a list",
			storedRepositories: []string{apiRepository},
			changes:            EditTaskDto{Repositories: &[]string{uiRepository}},
			wantRepositories:   []string{uiRepository},
		},
		{
			name:               "replacing a list with repeats",
			storedRepositories: []string{apiRepository},
			changes:            EditTaskDto{Repositories: &[]string{uiRepository, uiRepository}},
			wantRepositories:   []string{uiRepository},
		},
		{
			name:               "clearing a list",
			storedRepositories: []string{apiRepository},
			changes:            EditTaskDto{Repositories: &[]string{}},
			wantRepositories:   nil,
		},
		{
			name:             "adding to an empty list",
			changes:          EditTaskDto{AddRepositories: &[]string{apiRepository}},
			wantRepositories: []string{apiRepository},
		},
		{
			name:               "adding a path already on the list",
			storedRepositories: []string{apiRepository},
			changes:            EditTaskDto{AddRepositories: &[]string{apiRepository, uiRepository, uiRepository}},
			wantRepositories:   []string{apiRepository, uiRepository},
		},
		{
			name:        "adding nothing",
			changes:     EditTaskDto{AddRepositories: &[]string{}},
			wantErrText: []string{"name at least one repository to add"},
		},
		{
			name:               "removing one of several",
			storedRepositories: []string{apiRepository, uiRepository, docsRepository},
			changes:            EditTaskDto{RemoveRepositories: &[]string{uiRepository}},
			wantRepositories:   []string{apiRepository, docsRepository},
		},
		{
			name:               "removing the last one",
			storedRepositories: []string{apiRepository},
			changes:            EditTaskDto{RemoveRepositories: &[]string{apiRepository}},
			wantErrText:        []string{"leaves it on all repositories of the project"},
		},
		{
			name:               "removing one that is not listed",
			storedRepositories: []string{apiRepository},
			changes:            EditTaskDto{RemoveRepositories: &[]string{uiRepository}},
			wantErrText:        []string{`task 006684e3 does not list repository "ui"`},
		},
		{
			name:                "removing one the project no longer records",
			projectRepositories: []string{apiRepository},
			storedRepositories:  []string{apiRepository, uiRepository},
			changes:             EditTaskDto{RemoveRepositories: &[]string{uiRepository}},
			wantRepositories:    []string{apiRepository},
		},
		{
			name:        "removing nothing",
			changes:     EditTaskDto{RemoveRepositories: &[]string{}},
			wantErrText: []string{"name at least one repository to remove"},
		},
		{
			name: "replacing the list and adding",
			changes: EditTaskDto{
				Repositories:    &[]string{apiRepository},
				AddRepositories: &[]string{uiRepository},
			},
			wantErrText: []string{"the repository list", "the repositories to add"},
		},
		{
			name: "adding and removing",
			changes: EditTaskDto{
				AddRepositories:    &[]string{apiRepository},
				RemoveRepositories: &[]string{uiRepository},
			},
			wantErrText: []string{"the repositories to add", "the repositories to remove"},
		},
		{
			name:        "setting an unknown path",
			changes:     EditTaskDto{Repositories: &[]string{apiRepository, "web"}},
			wantErrText: []string{`no repository "web"`, "it has: api, ui, docs"},
		},
		{
			name:        "adding an unknown path",
			changes:     EditTaskDto{AddRepositories: &[]string{"web"}},
			wantErrText: []string{`no repository "web"`, "it has: api, ui, docs"},
		},
		{
			name:                "adding to a project that records no repositories",
			projectRepositories: []string{},
			changes:             EditTaskDto{AddRepositories: &[]string{apiRepository}},
			wantErrText:         []string{`no repository "api"`, "records no repositories"},
		},
		{
			name:                "the project directory in a project that records it",
			projectRepositories: []string{selfRepository},
			changes:             EditTaskDto{Repositories: &[]string{selfRepository}},
			wantRepositories:    []string{selfRepository},
		},
		{
			name:        "the project directory in a project that does not record it",
			changes:     EditTaskDto{Repositories: &[]string{selfRepository}},
			wantErrText: []string{`no repository "."`, "it has: api, ui, docs"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRepositories := testCase.projectRepositories
			if projectRepositories == nil {
				projectRepositories = multiRepositoryProject
			}
			stored := editableTask()
			stored.Repositories = slices.Clone(testCase.storedRepositories)
			repo := &fakeTaskRepo{tasks: []*Task{stored}}
			service := NewTaskService(repo, &fakeProgress{}, StatusDraft, projectRepositories)

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
				if !slices.Equal(stored.Repositories, testCase.storedRepositories) {
					t.Errorf("expected the repositories to stay %v, got %v", testCase.storedRepositories, stored.Repositories)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(edited.Repositories, testCase.wantRepositories) {
				t.Errorf("expected the repositories %#v, got %#v", testCase.wantRepositories, edited.Repositories)
			}
			if repo.writes != 1 {
				t.Errorf("expected one write, got %d", repo.writes)
			}
		})
	}
}

func TestTaskService_EditTask_RefusesRepositoriesWhileASessionWorks(t *testing.T) {
	refusal := errors.New("an agent is working on task 006684e3")
	stored := editableTask()
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, &fakeProgress{}, StatusDraft, multiRepositoryProject)

	_, err := service.EditTask(testProjectSlug, editableTaskID, EditTaskDto{AddRepositories: &[]string{apiRepository}}, &fakeSessionGuard{refusal: refusal})

	if !errors.Is(err, refusal) {
		t.Fatalf("expected the Session guard to refuse the edit, got %v", err)
	}
	if repo.writes != 0 || stored.Repositories != nil {
		t.Errorf("expected a refused edit to change nothing, got %d writes and %v", repo.writes, stored.Repositories)
	}
}

func TestTaskService_CreateTask_Repositories(t *testing.T) {
	cases := []struct {
		name                string
		projectRepositories []string
		repositories        []string
		parent              TaskID
		wantRepositories    []string
		// wantErrText are fragments a refusal must carry. A case without them
		// expects the task to be created.
		wantErrText []string
	}{
		{
			name: "no repositories",
		},
		{
			name:             "several repositories",
			repositories:     []string{apiRepository, docsRepository},
			wantRepositories: []string{apiRepository, docsRepository},
		},
		{
			name:             "repeats collapsing",
			repositories:     []string{apiRepository, apiRepository},
			wantRepositories: []string{apiRepository},
		},
		{
			name:         "an unknown path",
			repositories: []string{"web"},
			wantErrText:  []string{`no repository "web"`, "it has: api, ui, docs"},
		},
		{
			name:                "the project directory in a project that records it",
			projectRepositories: []string{selfRepository},
			repositories:        []string{selfRepository},
			wantRepositories:    []string{selfRepository},
		},
		{
			name:         "the project directory in a project that does not record it",
			repositories: []string{selfRepository},
			wantErrText:  []string{`no repository "."`},
		},
		{
			name:   "a subtask of a task that lists repositories",
			parent: migrationTaskID,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRepositories := testCase.projectRepositories
			if projectRepositories == nil {
				projectRepositories = multiRepositoryProject
			}
			parent := backlogTask(migrationTaskID, "Add the migration", StatusTodo)
			parent.Repositories = []string{apiRepository}
			repo := &fakeTaskRepo{tasks: []*Task{parent}}
			service := NewTaskService(repo, &fakeProgress{}, StatusDraft, projectRepositories)

			created, err := service.CreateTask(CreateTaskDto{
				Title:        "Wire the service",
				ProjectSlug:  testProjectSlug,
				Repositories: testCase.repositories,
				ParentTaskID: testCase.parent,
			})

			if testCase.wantErrText != nil {
				if err == nil {
					t.Fatal("expected the task to be refused")
				}
				for _, want := range testCase.wantErrText {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("expected the error to carry %q, got %q", want, err)
					}
				}
				if len(repo.tasks) != 1 {
					t.Errorf("expected a refused task to be left unwritten, got %d tasks", len(repo.tasks))
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(created.Repositories, testCase.wantRepositories) {
				t.Errorf("expected the repositories %#v, got %#v", testCase.wantRepositories, created.Repositories)
			}
		})
	}
}
