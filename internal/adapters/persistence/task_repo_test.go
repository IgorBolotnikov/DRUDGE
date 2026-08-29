package persistence

import (
	"os"
	"path/filepath"
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
		t.Fatalf("GetTask: %v", err)
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
		t.Fatalf("GetTask: %v", err)
	}

	if !found.CreatedAt.Equal(now.Truncate(time.Second)) {
		t.Errorf("created_at mismatch: expected %v, got %v", now.Truncate(time.Second), found.CreatedAt)
	}
	if !found.UpdatedAt.IsZero() {
		t.Errorf("expected updated_at to be zero, got %v", found.UpdatedAt)
	}
}
