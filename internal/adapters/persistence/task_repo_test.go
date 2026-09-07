package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

func setupTaskTestHome(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origCwd) })

	return dir, func() {}
}

func TestFileTaskRepository_CreateTask_CreatesFile(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	dto := task.CreateTaskDto{
		Title:       "Fix login bug",
		Description: "Users can't login with SSO",
		Status:      task.StatusTodo,
		TicketID:    "PROJ-123",
		ProjectSlug: "test-project",
		CreatedAt:   now,
	}

	tk, err := repo.CreateTask(dto)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if tk.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if tk.Title != "Fix login bug" {
		t.Errorf("expected title 'Fix login bug', got %q", tk.Title)
	}
	if tk.Status != task.StatusTodo {
		t.Errorf("expected status 'todo', got %q", tk.Status)
	}
	if tk.TicketID != "PROJ-123" {
		t.Errorf("expected ticket_id 'PROJ-123', got %q", tk.TicketID)
	}
	if tk.ProjectSlug != "test-project" {
		t.Errorf("expected project_slug 'test-project', got %q", tk.ProjectSlug)
	}
}

func TestFileTaskRepository_CreateTask_CreatesTasksDir(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	dto := task.CreateTaskDto{
		Title:       "New Task",
		Description: "Test description",
		Status:      task.StatusDraft,
		ProjectSlug: "test-project",
		CreatedAt:   now,
	}

	if _, err := repo.CreateTask(dto); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", "tasks")
	info, err := os.Stat(tasksDir)
	if err != nil {
		t.Fatalf("tasks directory not found: %v", err)
	}
	if !info.IsDir() {
		t.Error("tasks should be a directory")
	}
}

func TestFileTaskRepository_CreateTask_GeneratesUniqueIDs(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		dto := task.CreateTaskDto{
			Title:       "Task",
			Description: "Desc",
			Status:      task.StatusDraft,
			ProjectSlug: "test-project",
			CreatedAt:   now,
		}
		tk, err := repo.CreateTask(dto)
		if err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
		if ids[string(tk.ID)] {
			t.Fatalf("duplicate task ID: %s", tk.ID)
		}
		ids[string(tk.ID)] = true
	}
}

func TestFileTaskRepository_CreateTask_OmitsEmptyTicketID(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	dto := task.CreateTaskDto{
		Title:       "Independent Task",
		Description: "No ticket",
		Status:      task.StatusTodo,
		ProjectSlug: "test-project",
		CreatedAt:   now,
	}

	if _, err := repo.CreateTask(dto); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	files := make([]os.DirEntry, 0)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			files = append(files, e)
		}
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 task file, got %d", len(files))
	}

	data, err := os.ReadFile(filepath.Join(tasksDir, files[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(data) == "" {
		t.Fatal("file should not be empty")
	}
}

func TestFileTaskRepository_CreateTask_ContentSaved(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	desc := "This is the description\nWith multiple lines\nand paragraphs"
	dto := task.CreateTaskDto{
		Title:       "Task With Content",
		Description: desc,
		Status:      task.StatusDraft,
		ProjectSlug: "test-project",
		CreatedAt:   now,
	}

	if _, err := repo.CreateTask(dto); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	files := make([]os.DirEntry, 0)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			files = append(files, e)
		}
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 task file, got %d", len(files))
	}

	data, err := os.ReadFile(filepath.Join(tasksDir, files[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	_, content := common.ParseFrontMatter(string(data))
	if content != desc {
		t.Errorf("expected content %q, got %q", desc, content)
	}
}

func TestFileTaskRepository_ListTasks_Empty(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")

	tasks, err := repo.ListTasks("test-project")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestFileTaskRepository_ListTasks_ReturnsCreated(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	for _, title := range []string{"Task One", "Task Two", "Task Three"} {
		dto := task.CreateTaskDto{
			Title:       title,
			Description: "Desc",
			Status:      task.StatusDraft,
			ProjectSlug: "test-project",
			CreatedAt:   now,
		}
		if _, err := repo.CreateTask(dto); err != nil {
			t.Fatalf("CreateTask %s: %v", title, err)
		}
	}

	tasks, err := repo.ListTasks("test-project")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestFileTaskRepository_ListTasks_SkipsNonMdFiles(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	dto := task.CreateTaskDto{
		Title:       "Real Task",
		Description: "Desc",
		Status:      task.StatusDraft,
		ProjectSlug: "test-project",
		CreatedAt:   now,
	}
	if _, err := repo.CreateTask(dto); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", "tasks")
	if err := os.WriteFile(filepath.Join(tasksDir, "not-a-task.txt"), []byte("ignore"), common.DefaultFilePerm); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tasks, err := repo.ListTasks("test-project")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (skipped .txt), got %d", len(tasks))
	}
}

func TestFileTaskRepository_GetTask_Found(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	dto := task.CreateTaskDto{
		Title:       "Get Me",
		Description: "Find this",
		Status:      task.StatusInProgress,
		TicketID:    "PROJ-456",
		ProjectSlug: "test-project",
		CreatedAt:   now,
	}
	created, err := repo.CreateTask(dto)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	found, err := repo.GetTask("test-project", created.ID)
	if err != nil {
		t.Fatalf("FindTask: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, found.ID)
	}
	if found.Title != "Get Me" {
		t.Errorf("expected title 'Get Me', got %q", found.Title)
	}
	if found.Status != task.StatusInProgress {
		t.Errorf("expected status 'in-progress', got %q", found.Status)
	}
	if found.TicketID != "PROJ-456" {
		t.Errorf("expected ticket_id 'PROJ-456', got %q", found.TicketID)
	}
}

func TestFileTaskRepository_GetTask_NotFound(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")

	_, err := repo.GetTask("test-project", task.TaskID("nonexistent-id"))
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestFileTaskRepository_GetTask_ParsesTimestamps(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	now := time.Now().UTC()

	dto := task.CreateTaskDto{
		Title:       "Time Test",
		Description: "Check timestamps",
		Status:      task.StatusDraft,
		ProjectSlug: "test-project",
		CreatedAt:   now,
	}
	created, err := repo.CreateTask(dto)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	found, err := repo.GetTask("test-project", created.ID)
	if err != nil {
		t.Fatalf("FindTask: %v", err)
	}

	if !found.CreatedAt.Equal(now.Truncate(time.Second)) {
		t.Errorf("created_at mismatch: expected %v, got %v", now.Truncate(time.Second), found.CreatedAt)
	}
	if !found.UpdatedAt.IsZero() {
		t.Errorf("expected updated_at to be zero, got %v", found.UpdatedAt)
	}
}

func TestTaskFrontMatter_SessionRoundTrip(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	cases := []struct {
		name                 string
		sessionID            string
		wantSessionKeyInFile bool
	}{
		{name: "no session run yet"},
		{
			name:                 "the session the last run reported",
			sessionID:            "sess-abc123",
			wantSessionKeyInFile: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			written := &task.Task{
				ID:          "task-1",
				Title:       "Round Trip",
				Description: "Body stays put",
				Status:      task.StatusInProgress,
				ProjectSlug: "test-project",
				SessionID:   testCase.sessionID,
				CreatedAt:   time.Now().UTC(),
			}

			path := filepath.Join(home, "task.md")
			if err := common.WriteFileWithFrontMatter(path, taskFrontMatter(written), written.Description); err != nil {
				t.Fatalf("WriteFileWithFrontMatter: %v", err)
			}

			repo := NewFileTaskRepository("test-project")
			read, err := repo.parseTaskFromFile(path)
			if err != nil {
				t.Fatalf("parseTaskFromFile: %v", err)
			}

			if read.SessionID != written.SessionID {
				t.Errorf("expected session id %q, got %q", written.SessionID, read.SessionID)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			hasSessionKey := strings.Contains(string(data), metaKeySessionID)
			if hasSessionKey != testCase.wantSessionKeyInFile {
				t.Errorf("expected %s in the file: %v, got %v", metaKeySessionID, testCase.wantSessionKeyInFile, hasSessionKey)
			}
		})
	}
}

func TestFileTaskRepository_UpdateTask_PersistsSession(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	projectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	if err := common.EnsureDir(projectDir); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	created, err := repo.CreateTask(task.CreateTaskDto{
		Title:       "Fix login bug",
		Description: "Users can't login with SSO",
		Status:      task.StatusTodo,
		ProjectSlug: "test-project",
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	startedAt := time.Now().UTC().Truncate(time.Second)
	created.Status = task.StatusInProgress
	created.StartedAt = startedAt
	created.SessionID = "sess-abc123"

	if err := repo.UpdateTask("test-project", created); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	if created.UpdatedAt.IsZero() {
		t.Error("expected UpdateTask to stamp updated_at on the task")
	}

	reread, err := repo.GetTask("test-project", created.ID)
	if err != nil {
		t.Fatalf("FindTask: %v", err)
	}

	if reread.Status != task.StatusInProgress {
		t.Errorf("expected status %q, got %q", task.StatusInProgress, reread.Status)
	}
	if reread.SessionID != "sess-abc123" {
		t.Errorf("expected session id 'sess-abc123', got %q", reread.SessionID)
	}
	if !reread.StartedAt.Equal(startedAt) {
		t.Errorf("expected started at %v, got %v", startedAt, reread.StartedAt)
	}
	if reread.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be written to the file")
	}
	if reread.Description != created.Description {
		t.Errorf("expected the description to survive the update, got %q", reread.Description)
	}
}

func TestFileTaskRepository_UpdateTask_UnknownTask(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", TasksDirName)
	if err := common.EnsureDir(tasksDir); err != nil {
		t.Fatalf("ensure tasks dir: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	err := repo.UpdateTask("test-project", &task.Task{ID: "nope", Title: "Ghost"})
	if err == nil {
		t.Fatal("expected an error for a task that does not exist")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("expected the error to name the task id, got %q", err)
	}
}

func TestTaskFrontMatter_TimestampsRoundTrip(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Second)

	cases := []struct {
		name       string
		startedAt  time.Time
		finishedAt time.Time
	}{
		{name: "never started"},
		{name: "started", startedAt: now},
		{name: "started and finished", startedAt: now.Add(-time.Hour), finishedAt: now},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			written := &task.Task{
				ID:          "task-1",
				Title:       "Round Trip",
				Description: "Body stays put",
				Status:      task.StatusDone,
				ProjectSlug: "test-project",
				StartedAt:   testCase.startedAt,
				FinishedAt:  testCase.finishedAt,
				CreatedAt:   now,
			}

			path := filepath.Join(home, "task.md")
			if err := common.WriteFileWithFrontMatter(path, taskFrontMatter(written), written.Description); err != nil {
				t.Fatalf("WriteFileWithFrontMatter: %v", err)
			}

			repo := NewFileTaskRepository("test-project")
			read, err := repo.parseTaskFromFile(path)
			if err != nil {
				t.Fatalf("parseTaskFromFile: %v", err)
			}

			if !read.StartedAt.Equal(written.StartedAt) {
				t.Errorf("expected started at %v, got %v", written.StartedAt, read.StartedAt)
			}
			if !read.FinishedAt.Equal(written.FinishedAt) {
				t.Errorf("expected finished at %v, got %v", written.FinishedAt, read.FinishedAt)
			}
		})
	}
}

// writeTaskFile puts a task file with a chosen id in the project, so a test
// can pick ids that overlap. CreateTask generates random ones.
func writeTaskFile(t *testing.T, home string, id task.TaskID, title string) {
	t.Helper()
	tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", TasksDirName)
	if err := common.EnsureDir(tasksDir); err != nil {
		t.Fatalf("ensure tasks dir: %v", err)
	}

	written := &task.Task{
		ID:          id,
		Title:       title,
		Status:      task.StatusTodo,
		ProjectSlug: "test-project",
		CreatedAt:   time.Now().UTC(),
	}
	path := filepath.Join(tasksDir, taskFileName(id, title))
	if err := common.WriteFileWithFrontMatter(path, taskFrontMatter(written), ""); err != nil {
		t.Fatalf("write task file: %v", err)
	}
}

func TestFileTaskRepository_FindTask_ResolvesFullAndPartialIDs(t *testing.T) {
	const (
		login  = task.TaskID("006684e3-dbe9-4316-8aba-8a67a8f01f8f")
		logout = task.TaskID("00668f11-1111-4316-8aba-8a67a8f01f8f")
		ship   = task.TaskID("abcd1234-2222-4316-8aba-8a67a8f01f8f")
		short  = task.TaskID("abcd")
	)

	cases := []struct {
		name    string
		id      string
		wantID  task.TaskID
		wantErr string
	}{
		{name: "a full id", id: string(login), wantID: login},
		{name: "the prefix a listing prints", id: "006684e3", wantID: login},
		{name: "a prefix naming a single task", id: "abcd1", wantID: ship},
		{name: "an exact id that is also a prefix", id: "abcd", wantID: short},
		{name: "an uppercase prefix", id: "006684E3", wantID: login},
		{name: "a prefix matching several tasks", id: "00668", wantErr: "matches 2 tasks"},
		{name: "a prefix matching nothing", id: "9", wantErr: "not found"},
		{name: "an unknown id", id: "deadbeef", wantErr: "not found"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home, cleanup := setupTaskTestHome(t)
			defer cleanup()

			writeTaskFile(t, home, login, "Fix login")
			writeTaskFile(t, home, logout, "Fix logout")
			writeTaskFile(t, home, ship, "Ship it")
			writeTaskFile(t, home, short, "Short id")

			repo := NewFileTaskRepository("test-project")
			found, err := repo.FindTask("test-project", testCase.id)

			if testCase.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error for id %q", testCase.id)
				}
				if !strings.Contains(err.Error(), testCase.wantErr) {
					t.Errorf("expected error to mention %q, got %q", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if found.ID != testCase.wantID {
				t.Errorf("expected task %q, got %q", testCase.wantID, found.ID)
			}
		})
	}
}

func TestFileTaskRepository_FindTask_AmbiguousIDNamesEveryMatch(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	writeTaskFile(t, home, "006684e3-dbe9-4316-8aba-8a67a8f01f8f", "Fix login")
	writeTaskFile(t, home, "00668f11-1111-4316-8aba-8a67a8f01f8f", "Fix logout")

	repo := NewFileTaskRepository("test-project")
	_, err := repo.FindTask("test-project", "00668")
	if err == nil {
		t.Fatal("expected an error for an ambiguous id")
	}

	// A user picks the right task off this message, so every match has to be
	// named in full.
	for _, want := range []string{
		"006684e3-dbe9-4316-8aba-8a67a8f01f8f", "Fix login",
		"00668f11-1111-4316-8aba-8a67a8f01f8f", "Fix logout",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to name %q, got %q", want, err)
		}
	}
}

func TestFileTaskRepository_FindTask_RefusesAFileHoldingAnotherTask(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	writeTaskFile(t, home, "006684e3-dbe9-4316-8aba-8a67a8f01f8f", "Fix login")

	// A file renamed by hand puts the name and the front matter out of step.
	tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", TasksDirName)
	old := filepath.Join(tasksDir, taskFileName("006684e3-dbe9-4316-8aba-8a67a8f01f8f", "Fix login"))
	renamed := filepath.Join(tasksDir, taskFileName("99999999-0000-4000-8000-000000000000", "Fix login"))
	if err := os.Rename(old, renamed); err != nil {
		t.Fatalf("rename: %v", err)
	}

	repo := NewFileTaskRepository("test-project")
	if _, err := repo.FindTask("test-project", "99999999"); err == nil {
		t.Fatal("expected an error for a file whose name disagrees with its front matter")
	}
}

func TestFileTaskRepository_FindTask_RefusesAnEmptyID(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	// One task in the project, so an unguarded prefix search would match it.
	writeTaskFile(t, home, "006684e3-dbe9-4316-8aba-8a67a8f01f8f", "Fix login")

	repo := NewFileTaskRepository("test-project")
	if _, err := repo.FindTask("test-project", ""); !errors.Is(err, task.ErrNoTaskID) {
		t.Fatalf("expected %v, got %v", task.ErrNoTaskID, err)
	}
}

func TestFileTaskRepository_GetTask_TakesFullIDsOnly(t *testing.T) {
	const login = task.TaskID("006684e3-dbe9-4316-8aba-8a67a8f01f8f")

	// GetTask serves callers that already hold an id, so it matches the whole
	// id and nothing else. Resolving what a user typed is FindTask's job.
	cases := []struct {
		name    string
		id      task.TaskID
		wantErr bool
	}{
		{name: "the full id", id: login},
		{name: "the prefix a listing prints", id: "006684e3", wantErr: true},
		{name: "the full id in uppercase", id: "006684E3-DBE9-4316-8ABA-8A67A8F01F8F", wantErr: true},
		{name: "an empty id", id: "", wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home, cleanup := setupTaskTestHome(t)
			defer cleanup()
			writeTaskFile(t, home, login, "Fix login")

			repo := NewFileTaskRepository("test-project")
			found, err := repo.GetTask("test-project", testCase.id)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error for id %q", testCase.id)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if found.ID != login {
				t.Errorf("expected task %q, got %q", login, found.ID)
			}
		})
	}
}

func TestTaskFrontMatter_OutcomeRoundTrip(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	// Every case is what one recorded run left on a task.
	cases := []struct {
		name     string
		failed   bool
		result   string
		turns    int
		duration time.Duration
		costUSD  float64
		// wantKeysInFile are the front matter keys the file should carry. A
		// zero field is left out to have no empty entries in the file.
		wantKeysInFile []string
	}{
		{
			name:           "a run nothing was recorded for yet",
			wantKeysInFile: nil,
		},
		{
			name:     "a run that got the work done",
			result:   "Done",
			turns:    3,
			duration: 8664 * time.Millisecond,
			costUSD:  0.0695468,
			wantKeysInFile: []string{
				metaKeySessionResult,
				metaKeySessionTurns,
				metaKeySessionDuration,
				metaKeySessionCostUSD,
			},
		},
		{
			name:     "a run the agent flagged as an error",
			failed:   true,
			result:   "Could not build",
			turns:    2,
			duration: 4 * time.Second,
			costUSD:  0.02,
			wantKeysInFile: []string{
				metaKeySessionFailed,
				metaKeySessionResult,
				metaKeySessionTurns,
				metaKeySessionDuration,
				metaKeySessionCostUSD,
			},
		},
		{
			name:           "a result the agent wrote over several lines",
			result:         "Done:\n\n- built it\n---\n- tested it",
			turns:          7,
			wantKeysInFile: []string{metaKeySessionResult, metaKeySessionTurns},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			written := &task.Task{
				ID:          "task-1",
				Title:       "Round Trip",
				Description: "Body stays put",
				Status:      task.StatusDone,
				ProjectSlug: "test-project",
				CreatedAt:   time.Now().UTC(),

				SessionFailed:   testCase.failed,
				SessionResult:   testCase.result,
				SessionTurns:    testCase.turns,
				SessionDuration: testCase.duration,
				SessionCostUSD:  testCase.costUSD,
			}

			path := filepath.Join(home, "task.md")
			if err := common.WriteFileWithFrontMatter(path, taskFrontMatter(written), written.Description); err != nil {
				t.Fatalf("WriteFileWithFrontMatter: %v", err)
			}

			repo := NewFileTaskRepository("test-project")
			read, err := repo.parseTaskFromFile(path)
			if err != nil {
				t.Fatalf("parseTaskFromFile: %v", err)
			}

			if read.SessionFailed != written.SessionFailed {
				t.Errorf("expected the error flag %v, got %v", written.SessionFailed, read.SessionFailed)
			}
			if read.SessionResult != written.SessionResult {
				t.Errorf("expected result %q, got %q", written.SessionResult, read.SessionResult)
			}
			if read.SessionTurns != written.SessionTurns {
				t.Errorf("expected %d turns, got %d", written.SessionTurns, read.SessionTurns)
			}
			if read.SessionDuration != written.SessionDuration {
				t.Errorf("expected duration %s, got %s", written.SessionDuration, read.SessionDuration)
			}
			if read.SessionCostUSD != written.SessionCostUSD {
				t.Errorf("expected cost %v, got %v", written.SessionCostUSD, read.SessionCostUSD)
			}
			if read.Description != written.Description {
				t.Errorf("expected the body %q, got %q", written.Description, read.Description)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			for _, key := range outcomeMetaKeys {
				wanted := slices.Contains(testCase.wantKeysInFile, key)
				if strings.Contains(string(data), key) != wanted {
					t.Errorf("expected %s in the file: %v", key, wanted)
				}
			}
		})
	}
}

// outcomeMetaKeys are every front matter key of a recorded run outcome.
var outcomeMetaKeys = []string{
	metaKeySessionFailed,
	metaKeySessionResult,
	metaKeySessionTurns,
	metaKeySessionDuration,
	metaKeySessionCostUSD,
}

func TestTaskFrontMatter_VendorErrorRoundTrip(t *testing.T) {
	home, cleanup := setupTaskTestHome(t)
	defer cleanup()

	// Every case is why the vendor turned a task's last run away.
	cases := []struct {
		name        string
		vendorError string
		class       task.VendorErrorClass
		// wantKeysInFile are the front matter keys the file should carry. A
		// zero field is left out to have no empty entries in the file.
		wantKeysInFile []string
	}{
		{
			name:           "a run the vendor had no part in",
			wantKeysInFile: nil,
		},
		{
			name:           "credentials the vendor would not take",
			vendorError:    "Failed to authenticate: OAuth session expired",
			class:          task.VendorErrorAuth,
			wantKeysInFile: []string{metaKeyVendorError, metaKeyVendorErrorClass},
		},
		{
			name:           "a rate limit",
			vendorError:    "Rate limit exceeded",
			class:          task.VendorErrorRateLimit,
			wantKeysInFile: []string{metaKeyVendorError, metaKeyVendorErrorClass},
		},
		{
			name:           "an error the vendor wrote over several lines",
			vendorError:    "Refused:\n\n- token expired\n---\n- log in again",
			class:          task.VendorErrorUnknown,
			wantKeysInFile: []string{metaKeyVendorError, metaKeyVendorErrorClass},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			written := &task.Task{
				ID:          "task-1",
				Title:       "Round Trip",
				Description: "Body stays put",
				Status:      task.StatusTodo,
				ProjectSlug: "test-project",
				CreatedAt:   time.Now().UTC(),

				VendorError:      testCase.vendorError,
				VendorErrorClass: testCase.class,
			}

			path := filepath.Join(home, "task.md")
			if err := common.WriteFileWithFrontMatter(path, taskFrontMatter(written), written.Description); err != nil {
				t.Fatalf("WriteFileWithFrontMatter: %v", err)
			}

			repo := NewFileTaskRepository("test-project")
			read, err := repo.parseTaskFromFile(path)
			if err != nil {
				t.Fatalf("parseTaskFromFile: %v", err)
			}

			if read.VendorError != written.VendorError {
				t.Errorf("expected vendor error %q, got %q", written.VendorError, read.VendorError)
			}
			if read.VendorErrorClass != written.VendorErrorClass {
				t.Errorf("expected vendor error class %q, got %q", written.VendorErrorClass, read.VendorErrorClass)
			}
			if read.Description != written.Description {
				t.Errorf("expected the body %q, got %q", written.Description, read.Description)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			for _, key := range vendorErrorMetaKeys {
				wanted := slices.Contains(testCase.wantKeysInFile, key)
				// One key is a prefix of the other, so the entry is matched
				// with the colon that follows it.
				if strings.Contains(string(data), key+":") != wanted {
					t.Errorf("expected %s in the file: %v", key, wanted)
				}
			}
		})
	}
}

// vendorErrorMetaKeys are every front matter key of a recorded vendor refusal.
var vendorErrorMetaKeys = []string{
	metaKeyVendorError,
	metaKeyVendorErrorClass,
}
