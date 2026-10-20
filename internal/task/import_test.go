package task

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const otherProjectSlug = "other"

func archivedTask(projectSlug string) *Task {
	return &Task{
		ID:          "11111111-dbe9-4316-8aba-8a67a8f01f8f",
		Title:       "Add retry",
		Description: "Retry uploads",
		Status:      StatusTodo,
		ProjectSlug: projectSlug,
		BlockedBy:   []TaskID{"99999999-dbe9-4316-8aba-8a67a8f01f8f"},
		CreatedAt:   time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
	}
}

func inProgressTask(projectSlug string) *Task {
	inProgress := archivedTask(projectSlug)
	inProgress.Status = StatusInProgress
	inProgress.SessionID = "session-1"
	inProgress.StartedAt = time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	inProgress.Landings = map[string]Landing{"api": {Branch: "drudge/11111111-add-retry", Base: "abc123"}}
	return inProgress
}

func storedCopy() *Task {
	stored := archivedTask(testProjectSlug)
	stored.Title = "Add retry, edited here"
	return stored
}

func TestTaskService_ImportTasks(t *testing.T) {
	cases := []struct {
		name         string
		manifestSlug string
		archived     *Task
		stored       []*Task
		wantTasks    []*Task
		wantEvents   []any
	}{
		{
			name:         "a task the project does not have",
			manifestSlug: testProjectSlug,
			archived:     archivedTask(testProjectSlug),
			wantTasks:    []*Task{archivedTask(testProjectSlug)},
			wantEvents: []any{
				TaskImported{Task: archivedTask(testProjectSlug)},
				TasksImported{ProjectSlug: testProjectSlug, Added: 1},
			},
		},
		{
			name:         "a task whose id the project has",
			manifestSlug: testProjectSlug,
			archived:     archivedTask(testProjectSlug),
			stored:       []*Task{storedCopy()},
			wantTasks:    []*Task{storedCopy()},
			wantEvents: []any{
				TaskImportSkipped{Task: archivedTask(testProjectSlug)},
				TasksImported{ProjectSlug: testProjectSlug, Skipped: 1},
			},
		},
		{
			name:         "a task in progress",
			manifestSlug: testProjectSlug,
			archived:     inProgressTask(testProjectSlug),
			wantTasks:    []*Task{inProgressTask(testProjectSlug)},
			wantEvents: []any{
				TaskImported{Task: inProgressTask(testProjectSlug)},
				TasksImported{ProjectSlug: testProjectSlug, Added: 1},
			},
		},
		{
			name:         "a task of another project",
			manifestSlug: otherProjectSlug,
			archived:     archivedTask(otherProjectSlug),
			wantTasks:    []*Task{archivedTask(testProjectSlug)},
			wantEvents: []any{
				ProjectSlugRewritten{ArchiveSlug: otherProjectSlug, ProjectSlug: testProjectSlug},
				TaskImported{Task: archivedTask(testProjectSlug)},
				TasksImported{ProjectSlug: testProjectSlug, Added: 1},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{
				tasks:    testCase.stored,
				manifest: ArchiveManifest{FormatVersion: ArchiveFormatVersion, ProjectSlug: testCase.manifestSlug},
				files:    []TaskFile{{Name: "11111111 Add retry.md", Task: testCase.archived}},
			}
			progress := &fakeProgress{}
			service := NewTaskService(repo, progress, "", nil)

			if err := service.ImportTasks(ImportTasksDto{ProjectSlug: testProjectSlug, Path: "tasks.zip"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(repo.tasks, testCase.wantTasks) {
				t.Errorf("expected tasks %+v, got %+v", testCase.wantTasks, repo.tasks)
			}
			if !reflect.DeepEqual(progress.events, testCase.wantEvents) {
				t.Errorf("expected events %+v, got %+v", testCase.wantEvents, progress.events)
			}
		})
	}
}

func TestTaskService_ImportTasks_Refusals(t *testing.T) {
	cases := []struct {
		name     string
		manifest ArchiveManifest
		readErr  error
		// wantErrText is a fragment the refusal must carry.
		wantErrText string
	}{
		{
			name:        "a broken archive",
			readErr:     &BrokenArchiveError{Reason: "it has no manifest.json"},
			wantErrText: "could not import tasks.zip: it has no manifest.json, a fresh export on the source device fixes it",
		},
		{
			name:        "an unknown format version",
			manifest:    ArchiveManifest{FormatVersion: ArchiveFormatVersion + 1, ProjectSlug: testProjectSlug},
			wantErrText: "it has archive format version 2 and drg reads version 1, a fresh export on the source device fixes it",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{
				manifest: testCase.manifest,
				files:    []TaskFile{{Name: "11111111 Add retry.md", Task: archivedTask(testProjectSlug)}},
				readErr:  testCase.readErr,
			}
			progress := &fakeProgress{}
			service := NewTaskService(repo, progress, "", nil)

			err := service.ImportTasks(ImportTasksDto{ProjectSlug: testProjectSlug, Path: "tasks.zip"})

			if err == nil || !strings.Contains(err.Error(), testCase.wantErrText) {
				t.Fatalf("expected an error containing %q, got %v", testCase.wantErrText, err)
			}
			if repo.writes != 0 || len(repo.tasks) != 0 {
				t.Errorf("expected nothing written, got %d writes and tasks %+v", repo.writes, repo.tasks)
			}
			if len(progress.events) != 0 {
				t.Errorf("expected no events, got %+v", progress.events)
			}
		})
	}
}
