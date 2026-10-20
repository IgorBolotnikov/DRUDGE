package persistence

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const (
	loginTaskID  task.TaskID = "006684e3-dbe9-4316-8aba-8a67a8f01f8f"
	testManifest             = `{"formatVersion":1,"projectSlug":"test-project","exportedAt":"2026-10-18T14:30:05Z"}`
)

var testArchiveManifest = task.ArchiveManifest{FormatVersion: 1, ProjectSlug: "test-project", ExportedAt: time.Date(2026, 10, 18, 14, 30, 5, 0, time.UTC)}

// recordedProgress records the events a service reports.
type recordedProgress struct {
	events []any
}

func (progress *recordedProgress) Report(event any) {
	progress.events = append(progress.events, event)
}

// writeZipArchive writes a zip archive holding entries at path.
func writeZipArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	archiveFile, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archiveFile.Close()

	archive := zip.NewWriter(archiveFile)
	for name, content := range entries {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
}

// fullTask is a task with every field set, the way a task looks after a run.
func fullTask(id task.TaskID, title string, projectSlug string) *task.Task {
	return &task.Task{
		ID:               id,
		Title:            title,
		Description:      "Users can't log in with SSO",
		Status:           task.StatusInProgress,
		StartedAt:        time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC),
		FinishedAt:       time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC),
		Repositories:     []string{"api"},
		TicketID:         "R-003-10",
		ParentTaskID:     "77777777-dbe9-4316-8aba-8a67a8f01f8f",
		ProjectSlug:      projectSlug,
		BlockedBy:        []task.TaskID{"88888888-dbe9-4316-8aba-8a67a8f01f8f"},
		PullRequests:     []string{"https://github.com/acme/api/pull/7"},
		SessionID:        "session-1",
		HasSessionFailed: true,
		SessionResult:    "Gave up",
		SessionTurns:     12,
		SessionDuration:  90 * time.Second,
		SessionCostUSD:   1.25,
		Stashes:          map[string]string{"api": "abc123"},
		Landings:         map[string]task.Landing{"api": {Branch: "drudge/006684e3-fix-login", Base: "def456", Head: "fed654", Commits: 2}},
		VendorError:      "OAuth token has expired",
		VendorErrorClass: task.VendorErrorAuth,
		CreatedAt:        time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
	}
}

func TestFileTaskRepository_WriteTask(t *testing.T) {
	refusal := errors.New("not this one")

	cases := []struct {
		name    string
		stored  *task.Task
		written *task.Task
		// decision is what decide answers.
		decision      error
		wantIsWritten bool
		wantErr       error
		// wantTask is the task stored once the write is over, and wantFile
		// the name of its file.
		wantTask *task.Task
		wantFile string
	}{
		{
			name:          "a task the project does not have",
			written:       fullTask(loginTaskID, "Fix login", "test-project"),
			wantIsWritten: true,
			wantTask:      fullTask(loginTaskID, "Fix login", "test-project"),
			wantFile:      loginTaskFile,
		},
		{
			name:          "a task the project has, under another title",
			stored:        fullTask(loginTaskID, "Old title", "test-project"),
			written:       fullTask(loginTaskID, "Fix login", "test-project"),
			wantIsWritten: true,
			wantTask:      fullTask(loginTaskID, "Fix login", "test-project"),
			wantFile:      loginTaskFile,
		},
		{
			name:     "a task decide keeps",
			stored:   fullTask(loginTaskID, "Old title", "test-project"),
			written:  fullTask(loginTaskID, "Fix login", "test-project"),
			decision: task.ErrTaskUnchanged,
			wantTask: fullTask(loginTaskID, "Old title", "test-project"),
			wantFile: string(loginTaskID) + " Old title.md",
		},
		{
			name:     "a task decide refuses",
			written:  fullTask(loginTaskID, "Fix login", "test-project"),
			decision: refusal,
			wantErr:  refusal,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupTaskTestHome(t)
			repo := NewFileTaskRepository("test-project")
			if testCase.stored != nil {
				if _, err := repo.WriteTask("test-project", testCase.stored, func(*task.Task) error { return nil }); err != nil {
					t.Fatalf("store task: %v", err)
				}
			}

			var gotStored *task.Task
			isWritten, err := repo.WriteTask("test-project", testCase.written, func(stored *task.Task) error {
				gotStored = stored
				return testCase.decision
			})

			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("expected error %v, got %v", testCase.wantErr, err)
			}
			if isWritten != testCase.wantIsWritten {
				t.Errorf("expected isWritten %v, got %v", testCase.wantIsWritten, isWritten)
			}
			if !reflect.DeepEqual(gotStored, testCase.stored) {
				t.Errorf("expected decide to get %+v, got %+v", testCase.stored, gotStored)
			}

			tasks, err := repo.ListTasks("test-project")
			if err != nil {
				t.Fatalf("list tasks: %v", err)
			}
			if testCase.wantTask == nil {
				if len(tasks) != 0 {
					t.Errorf("expected no tasks, got %+v", tasks)
				}
				return
			}
			if len(tasks) != 1 || !reflect.DeepEqual(tasks[0], testCase.wantTask) {
				t.Errorf("expected only task %+v, got %+v", testCase.wantTask, tasks)
			}
			files, err := repo.taskFiles()
			if err != nil {
				t.Fatalf("list task files: %v", err)
			}
			if len(files) != 1 || filepath.Base(files[0].path) != testCase.wantFile {
				t.Errorf("expected only file %s, got %+v", testCase.wantFile, files)
			}
		})
	}
}

func TestFileTaskRepository_ReadTaskArchive(t *testing.T) {
	cases := []struct {
		name string
		// entries are the entries of the archive. A nil map writes a file
		// that is not a zip.
		entries        map[string]string
		wantManifest   task.ArchiveManifest
		wantFiles      map[string]task.TaskStatus
		wantBadEntries []task.BadArchiveEntry
		// wantErrText is a fragment a refusal must carry.
		wantErrText string
	}{
		{
			name:         "a few tasks",
			entries:      map[string]string{ManifestFileName: testManifest, loginTaskFile: loginTask, shipTaskFile: shipTask},
			wantManifest: testArchiveManifest,
			wantFiles:    map[string]task.TaskStatus{loginTaskFile: task.StatusDone, shipTaskFile: task.StatusDraft},
		},
		{
			name:        "not a zip",
			wantErrText: "it is not a zip archive",
		},
		{
			name:        "no manifest",
			entries:     map[string]string{loginTaskFile: loginTask},
			wantErrText: "it has no manifest.json",
		},
		{
			name:        "a manifest that is not JSON",
			entries:     map[string]string{ManifestFileName: "{formatVersion"},
			wantErrText: "its manifest.json does not parse",
		},
		{
			name:        "a manifest with no format version",
			entries:     map[string]string{ManifestFileName: `{"projectSlug":"test-project"}`},
			wantErrText: "its manifest.json has no format version",
		},
		{
			name:        "a manifest with no project",
			entries:     map[string]string{ManifestFileName: `{"formatVersion":1}`},
			wantErrText: "its manifest.json names no project",
		},
		{
			name:         "an entry in a directory",
			entries:      map[string]string{ManifestFileName: testManifest, "tasks/" + loginTaskFile: loginTask, shipTaskFile: shipTask},
			wantManifest: testArchiveManifest,
			wantFiles:    map[string]task.TaskStatus{shipTaskFile: task.StatusDraft},
			wantBadEntries: []task.BadArchiveEntry{
				{Name: "tasks/" + loginTaskFile, Reason: "it is neither a task file nor the manifest.json"},
			},
		},
		{
			name:         "an entry outside the archive",
			entries:      map[string]string{ManifestFileName: testManifest, "../" + loginTaskFile: loginTask},
			wantManifest: testArchiveManifest,
			wantFiles:    map[string]task.TaskStatus{},
			wantBadEntries: []task.BadArchiveEntry{
				{Name: "../" + loginTaskFile, Reason: "its path points outside the archive"},
			},
		},
		{
			name:         "a task file holding another task",
			entries:      map[string]string{ManifestFileName: testManifest, shipTaskFile: loginTask},
			wantManifest: testArchiveManifest,
			wantFiles:    map[string]task.TaskStatus{},
			wantBadEntries: []task.BadArchiveEntry{
				{Name: shipTaskFile, Reason: "it does not parse: task file " + shipTaskFile + " holds task " + string(loginTaskID) + ", rename it back or fix its id"},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tasks.zip")
			if testCase.entries == nil {
				if err := os.WriteFile(path, []byte("not a zip"), common.DefaultFilePerm); err != nil {
					t.Fatal(err)
				}
			} else {
				writeZipArchive(t, path, testCase.entries)
			}

			archive, err := NewFileTaskRepository("test-project").ReadTaskArchive(path)

			if testCase.wantErrText != "" {
				var broken *task.BrokenArchiveError
				if !errors.As(err, &broken) || !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Fatalf("expected a broken archive error containing %q, got %v", testCase.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if archive.Manifest != testCase.wantManifest {
				t.Errorf("expected manifest %+v, got %+v", testCase.wantManifest, archive.Manifest)
			}
			gotFiles := map[string]task.TaskStatus{}
			for _, file := range archive.Files {
				gotFiles[file.Name] = file.Task.Status
				if string(file.Content) != testCase.entries[file.Name] {
					t.Errorf("expected %s read byte for byte, got %q", file.Name, file.Content)
				}
			}
			if !reflect.DeepEqual(gotFiles, testCase.wantFiles) {
				t.Errorf("expected files %v, got %v", testCase.wantFiles, gotFiles)
			}
			if !reflect.DeepEqual(archive.BadEntries, testCase.wantBadEntries) {
				t.Errorf("expected bad entries %+v, got %+v", testCase.wantBadEntries, archive.BadEntries)
			}
		})
	}
}

func TestTaskService_ImportTasks_RefusesBrokenArchives(t *testing.T) {
	cases := []struct {
		name string
		// entries are the entries of the archive. A nil map writes a file
		// that is not a zip.
		entries     map[string]string
		wantErrText string
	}{
		{
			name:        "not a zip",
			wantErrText: "it is not a zip archive",
		},
		{
			name:        "no manifest",
			entries:     map[string]string{loginTaskFile: loginTask},
			wantErrText: "it has no manifest.json",
		},
		{
			name:        "a malformed manifest",
			entries:     map[string]string{ManifestFileName: "{formatVersion", loginTaskFile: loginTask},
			wantErrText: "its manifest.json does not parse",
		},
		{
			name:        "an unknown format version",
			entries:     map[string]string{ManifestFileName: `{"formatVersion":99,"projectSlug":"test-project"}`, loginTaskFile: loginTask},
			wantErrText: "it has archive format version 99",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home, _ := setupTaskTestHome(t)
			path := filepath.Join(t.TempDir(), "tasks.zip")
			if testCase.entries == nil {
				if err := os.WriteFile(path, []byte("not a zip"), common.DefaultFilePerm); err != nil {
					t.Fatal(err)
				}
			} else {
				writeZipArchive(t, path, testCase.entries)
			}
			progress := &recordedProgress{}
			service := task.NewTaskService(NewFileTaskRepository("test-project"), progress, "", nil)

			err := service.ImportTasks(task.ImportTasksDto{ProjectSlug: "test-project", Path: path})

			if err == nil || !strings.Contains(err.Error(), testCase.wantErrText) || !strings.Contains(err.Error(), "a fresh export on the source device fixes it") {
				t.Fatalf("expected an error containing %q and the fresh export advice, got %v", testCase.wantErrText, err)
			}
			tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", TasksDirName)
			if _, err := os.Stat(tasksDir); !os.IsNotExist(err) {
				t.Errorf("expected no tasks directory, got %v", err)
			}
			if len(progress.events) != 0 {
				t.Errorf("expected no events, got %+v", progress.events)
			}
		})
	}
}

// badEntryCases are archives that hold the login task and one bad entry.
var badEntryCases = []struct {
	name         string
	badEntryName string
	badEntry     string
	// wantReason is a fragment the warning about the bad entry carries.
	wantReason string
}{
	{
		name:         "a task file that does not parse",
		badEntryName: shipTaskFile,
		badEntry:     "Ship it without front matter\n",
		wantReason:   "it does not parse: task file " + shipTaskFile + ` has no "id" in its front matter`,
	},
	{
		name:         "a task with an unknown status",
		badEntryName: shipTaskFile,
		badEntry:     strings.Replace(shipTask, "status: draft", "status: someday", 1),
		wantReason:   `it has unknown status "someday"`,
	},
	{
		name:         "a task with an id that is not a UUID",
		badEntryName: "ship-it Ship it.md",
		badEntry:     strings.Replace(shipTask, "id: abcd1234-2222-4316-8aba-8a67a8f01f8f", "id: ship-it", 1),
		wantReason:   `its id "ship-it" is not a UUID`,
	},
	{
		name:         "a path outside the archive",
		badEntryName: "../" + shipTaskFile,
		badEntry:     shipTask,
		wantReason:   "its path points outside the archive",
	},
	{
		name:         "a stray file",
		badEntryName: "notes.txt",
		badEntry:     "Ship it on Friday\n",
		wantReason:   "it is neither a task file nor the manifest.json",
	},
}

func TestTaskService_ImportTasks_SkipsBadEntries(t *testing.T) {
	for _, testCase := range badEntryCases {
		t.Run(testCase.name, func(t *testing.T) {
			home, _ := setupTaskTestHome(t)
			path := filepath.Join(t.TempDir(), "tasks.zip")
			writeZipArchive(t, path, map[string]string{
				ManifestFileName:      testManifest,
				loginTaskFile:         loginTask,
				testCase.badEntryName: testCase.badEntry,
			})
			repo := NewFileTaskRepository("test-project")
			progress := &recordedProgress{}

			err := task.NewTaskService(repo, progress, "", nil).ImportTasks(task.ImportTasksDto{ProjectSlug: "test-project", Path: path})

			wantErrText := "skipped 1 entry of " + path + " as bad, a fresh export on the source device fixes them"
			if err == nil || err.Error() != wantErrText {
				t.Fatalf("expected error %q, got %v", wantErrText, err)
			}
			if tasks := listTasksByID(t, repo); len(tasks) != 1 || tasks[0].ID != loginTaskID {
				t.Errorf("expected only task %s, got %+v", loginTaskID, tasks)
			}
			tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", TasksDirName)
			if _, err := os.Stat(filepath.Join(tasksDir, testCase.badEntryName)); !os.IsNotExist(err) {
				t.Errorf("expected nothing written for the bad entry, got %v", err)
			}

			var warnings []task.BadArchiveEntry
			for _, event := range progress.events {
				if warning, ok := event.(task.BadArchiveEntry); ok {
					warnings = append(warnings, warning)
				}
			}
			if len(warnings) != 1 || warnings[0].Name != testCase.badEntryName || !strings.Contains(warnings[0].Reason, testCase.wantReason) {
				t.Errorf("expected one warning about %s containing %q, got %+v", testCase.badEntryName, testCase.wantReason, warnings)
			}
			wantSummary := task.TasksImported{ProjectSlug: "test-project", Added: 1, SkippedAsBad: 1}
			if summary := progress.events[len(progress.events)-1]; summary != wantSummary {
				t.Errorf("expected summary %+v, got %+v", wantSummary, summary)
			}
		})
	}
}

func TestTaskService_ImportTasks_DryRunOfBadEntries(t *testing.T) {
	for _, testCase := range badEntryCases {
		t.Run(testCase.name, func(t *testing.T) {
			home, _ := setupTaskTestHome(t)
			path := filepath.Join(t.TempDir(), "tasks.zip")
			writeZipArchive(t, path, map[string]string{
				ManifestFileName:      testManifest,
				loginTaskFile:         loginTask,
				testCase.badEntryName: testCase.badEntry,
			})
			repo := NewFileTaskRepository("test-project")
			service := func(progress common.Progress) *task.TaskService { return task.NewTaskService(repo, progress, "", nil) }
			before := snapshotDir(t, home)

			dryRun := &recordedProgress{}
			dryRunErr := service(dryRun).ImportTasks(task.ImportTasksDto{ProjectSlug: "test-project", Path: path, IsDryRun: true})

			if after := snapshotDir(t, home); !reflect.DeepEqual(after, before) {
				t.Fatalf("expected the dry run to leave %v as it was, got %v", before, after)
			}

			realRun := &recordedProgress{}
			realErr := service(realRun).ImportTasks(task.ImportTasksDto{ProjectSlug: "test-project", Path: path})

			if dryRunErr == nil || realErr == nil || dryRunErr.Error() != realErr.Error() {
				t.Errorf("expected the dry run to fail like the import with %v, got %v", realErr, dryRunErr)
			}
			wantEvents := []any{task.ImportDryRunStarted{ProjectSlug: "test-project", Path: path}}
			for _, event := range realRun.events {
				if summary, ok := event.(task.TasksImported); ok {
					summary.IsDryRun = true
					event = summary
				}
				wantEvents = append(wantEvents, event)
			}
			if !reflect.DeepEqual(dryRun.events, wantEvents) {
				t.Errorf("expected events %+v, got %+v", wantEvents, dryRun.events)
			}
		})
	}
}

// snapshotDir reads every file under root, keyed by its path.
func snapshotDir(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			snapshot[path] = ""
			return err
		}
		content, err := os.ReadFile(path)
		snapshot[path] = string(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestTaskService_ImportTasks_RoundTrip(t *testing.T) {
	setupTaskTestHome(t)
	exported := []*task.Task{
		fullTask(loginTaskID, "Fix login", "alpha"),
		{
			ID:          "abcd1234-2222-4316-8aba-8a67a8f01f8f",
			Title:       "Ship it",
			Status:      task.StatusDraft,
			ProjectSlug: "alpha",
			CreatedAt:   time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC),
		},
	}
	source := NewFileTaskRepository("alpha")
	for _, stored := range exported {
		if _, err := source.WriteTask("alpha", stored, func(*task.Task) error { return nil }); err != nil {
			t.Fatalf("store task: %v", err)
		}
	}
	path := filepath.Join(t.TempDir(), "tasks.zip")
	if _, err := task.NewTaskService(source, &recordedProgress{}, "", nil).ExportTasks(task.ExportTasksDto{ProjectSlug: "alpha", Path: path}); err != nil {
		t.Fatalf("export: %v", err)
	}

	target := NewFileTaskRepository("beta")
	firstImport := &recordedProgress{}
	if err := task.NewTaskService(target, firstImport, "", nil).ImportTasks(task.ImportTasksDto{ProjectSlug: "beta", Path: path}); err != nil {
		t.Fatalf("first import: %v", err)
	}

	wantTasks := make([]*task.Task, 0, len(exported))
	for _, stored := range exported {
		moved := *stored
		moved.ProjectSlug = "beta"
		wantTasks = append(wantTasks, &moved)
	}
	imported := listTasksByID(t, target)
	if !reflect.DeepEqual(imported, wantTasks) {
		t.Errorf("expected tasks %+v, got %+v", wantTasks, imported)
	}
	wantSummary := task.TasksImported{ProjectSlug: "beta", Added: 2}
	if summary := firstImport.events[len(firstImport.events)-1]; summary != wantSummary {
		t.Errorf("expected summary %+v, got %+v", wantSummary, summary)
	}

	secondImport := &recordedProgress{}
	if err := task.NewTaskService(target, secondImport, "", nil).ImportTasks(task.ImportTasksDto{ProjectSlug: "beta", Path: path}); err != nil {
		t.Fatalf("second import: %v", err)
	}
	if again := listTasksByID(t, target); !reflect.DeepEqual(again, imported) {
		t.Errorf("expected the second import to change nothing, got %+v", again)
	}
	wantSummary = task.TasksImported{ProjectSlug: "beta", SkippedAsOlder: 1, SkippedAsInProgress: 1}
	if summary := secondImport.events[len(secondImport.events)-1]; summary != wantSummary {
		t.Errorf("expected summary %+v, got %+v", wantSummary, summary)
	}

	returnPath := filepath.Join(t.TempDir(), "tasks.zip")
	if _, err := task.NewTaskService(target, &recordedProgress{}, "", nil).ExportTasks(task.ExportTasksDto{ProjectSlug: "beta", Path: returnPath}); err != nil {
		t.Fatalf("return export: %v", err)
	}
	returnImport := &recordedProgress{}
	if err := task.NewTaskService(source, returnImport, "", nil).ImportTasks(task.ImportTasksDto{ProjectSlug: "alpha", Path: returnPath}); err != nil {
		t.Fatalf("return import: %v", err)
	}
	if returned := listTasksByID(t, source); !reflect.DeepEqual(returned, exported) {
		t.Errorf("expected the return import to change nothing, got %+v", returned)
	}
	wantSummary = task.TasksImported{ProjectSlug: "alpha", SkippedAsOlder: 1, SkippedAsInProgress: 1}
	if summary := returnImport.events[len(returnImport.events)-1]; summary != wantSummary {
		t.Errorf("expected summary %+v, got %+v", wantSummary, summary)
	}
}

func TestTaskService_ImportTasks_RenamesUpdatedTask(t *testing.T) {
	setupTaskTestHome(t)
	stored := fullTask(loginTaskID, "Old title", "test-project")
	stored.Status = task.StatusTodo
	repo := NewFileTaskRepository("test-project")
	if _, err := repo.WriteTask("test-project", stored, func(*task.Task) error { return nil }); err != nil {
		t.Fatalf("store task: %v", err)
	}

	edited := *stored
	edited.Title = "Fix login"
	edited.UpdatedAt = stored.UpdatedAt.Add(time.Hour)
	content := common.FormatFrontMatter(taskFrontMatter(&edited)) + edited.Description
	path := filepath.Join(t.TempDir(), "tasks.zip")
	writeZipArchive(t, path, map[string]string{ManifestFileName: testManifest, loginTaskFile: content})

	if err := task.NewTaskService(repo, &recordedProgress{}, "", nil).ImportTasks(task.ImportTasksDto{ProjectSlug: "test-project", Path: path}); err != nil {
		t.Fatalf("import: %v", err)
	}

	if tasks := listTasksByID(t, repo); len(tasks) != 1 || !reflect.DeepEqual(tasks[0], &edited) {
		t.Errorf("expected only task %+v, got %+v", &edited, tasks)
	}
	files, err := repo.taskFiles()
	if err != nil {
		t.Fatalf("list task files: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0].path) != loginTaskFile {
		t.Errorf("expected only file %s, got %+v", loginTaskFile, files)
	}
}

// listTasksByID lists the tasks of a project ordered by id.
func listTasksByID(t *testing.T, repo *FileTaskRepository) []*task.Task {
	t.Helper()
	tasks, err := repo.ListTasks(repo.Project)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	slices.SortFunc(tasks, func(left, right *task.Task) int { return strings.Compare(string(left.ID), string(right.ID)) })
	return tasks
}
