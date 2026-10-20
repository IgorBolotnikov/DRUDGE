package task

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTaskService_ExportTasks(t *testing.T) {
	exportedAt := time.Date(2026, 10, 18, 14, 30, 5, 0, time.UTC)
	taskFiles := []TaskFile{
		{Name: "11111111 Add retry.md", Content: []byte("---\nid: 11111111\nstatus: todo\n---\nRetry uploads"), Task: &Task{ID: "11111111", Status: StatusTodo}},
		{Name: "22222222 Fix login.md", Content: []byte("---\nid: 22222222\nstatus: done\n---\nFix SSO"), Task: &Task{ID: "22222222", Status: StatusDone}},
		{Name: "33333333 Draft idea.md", Content: []byte("---\nid: 33333333\nstatus: draft\n---\n"), Task: &Task{ID: "33333333", Status: StatusDraft}},
	}
	wantManifest := ArchiveManifest{FormatVersion: ArchiveFormatVersion, ProjectSlug: testProjectSlug, ExportedAt: exportedAt}

	cases := []struct {
		name          string
		path          string
		files         []TaskFile
		readErr       error
		existingPaths []string
		wantPath      string
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects the archive to be written.
		wantErrText string
	}{
		{
			name:     "a few tasks under the default name",
			files:    taskFiles,
			wantPath: "demo-3-tasks-20261018-143005.zip",
		},
		{
			name:     "a single task under the default name",
			files:    taskFiles[:1],
			wantPath: "demo-1-tasks-20261018-143005.zip",
		},
		{
			name:     "an output path",
			path:     "backups/tasks.zip",
			files:    taskFiles,
			wantPath: "backups/tasks.zip",
		},
		{
			name:          "an output path that already exists",
			path:          "tasks.zip",
			files:         taskFiles,
			existingPaths: []string{"tasks.zip"},
			wantErrText:   "tasks.zip already exists",
		},
		{
			name:          "a default name that already exists",
			files:         taskFiles,
			existingPaths: []string{"demo-3-tasks-20261018-143005.zip"},
			wantErrText:   "demo-3-tasks-20261018-143005.zip already exists",
		},
		{
			name:        "a broken task file",
			files:       taskFiles,
			readErr:     errors.New("task file 44444444 Broken.md holds task 55555555, rename it back or fix its id"),
			wantErrText: "task file 44444444 Broken.md holds task 55555555",
		},
		{
			name:        "a project with no tasks",
			wantErrText: "project demo has no tasks to export",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{files: testCase.files, readErr: testCase.readErr, existingPaths: testCase.existingPaths}
			service := NewTaskService(repo, &fakeProgress{}, "", nil)

			exported, err := service.ExportTasks(ExportTasksDto{ProjectSlug: testProjectSlug, Path: testCase.path, ExportedAt: exportedAt})

			if testCase.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Fatalf("expected an error containing %q, got %v", testCase.wantErrText, err)
				}
				if repo.archive != nil {
					t.Errorf("expected no archive written, got %s", repo.archive.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantExported := ExportedTasks{Path: testCase.wantPath, Count: len(testCase.files)}
			if exported != wantExported {
				t.Errorf("expected %+v, got %+v", wantExported, exported)
			}
			if repo.archive == nil {
				t.Fatal("expected an archive written")
			}
			if repo.archive.path != testCase.wantPath {
				t.Errorf("expected the archive at %s, got %s", testCase.wantPath, repo.archive.path)
			}
			if repo.archive.manifest != wantManifest {
				t.Errorf("expected manifest %+v, got %+v", wantManifest, repo.archive.manifest)
			}
			if !reflect.DeepEqual(repo.archive.files, testCase.files) {
				t.Errorf("expected every task file in the archive as read, got %+v", repo.archive.files)
			}
		})
	}
}
