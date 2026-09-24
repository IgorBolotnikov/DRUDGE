package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/project"
)

func newTestRepo(t *testing.T) (*FileProjectRepository, string) {
	t.Helper()
	dir := t.TempDir()
	return NewFileProjectRepository(dir), dir
}

func newTestDto(dir, name, slug string) project.CreateProjectDto {
	return project.CreateProjectDto{
		Name:      name,
		Slug:      slug,
		Location:  filepath.Join(dir, slug),
		CreatedAt: time.Now().UTC(),
	}
}

func TestFileProjectRepository_Project_DeleteProject_DoesNotExist(t *testing.T) {
	repo, _ := newTestRepo(t)

	err := repo.DeleteProject("nonexistent")
	if err != nil {
		t.Errorf("expected no error for nonexistent project: %v", err)
	}
}

func TestFileProjectRepository_Project_DeleteProject_RemovesDir(t *testing.T) {
	repo, dir := newTestRepo(t)

	dto := newTestDto(dir, "Delete Test", "delete-test")
	proj, err := repo.CreateProject(dto)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if proj.Slug != "delete-test" {
		t.Errorf("expected slug 'delete-test', got %q", proj.Slug)
	}

	if err := repo.DeleteProject("delete-test"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	_, err = os.Stat(filepath.Join(dir, "delete-test"))
	if !os.IsNotExist(err) {
		t.Error("project dir should be deleted")
	}
}

func TestFileProjectRepository_Project_ListProjects_EmptyDir(t *testing.T) {
	repo, _ := newTestRepo(t)

	projects, err := repo.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestFileProjectRepository_Project_ListProjects_NoProjectsDir(t *testing.T) {
	repo := NewFileProjectRepository(filepath.Join(t.TempDir(), "projects"))

	projects, err := repo.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestFileProjectRepository_Project_ListProjects_Single(t *testing.T) {
	repo, dir := newTestRepo(t)

	dto := newTestDto(dir, "One Project", "one-project")
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	projects, err := repo.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	if projects[0].Slug != "one-project" || projects[0].Name != "One Project" {
		t.Errorf("unexpected project: %+v", projects[0])
	}
}

func TestFileProjectRepository_Project_ListProjects_Multiple(t *testing.T) {
	repo, dir := newTestRepo(t)

	for i := 1; i <= 3; i++ {
		dto := newTestDto(dir, fmt.Sprintf("Project %d", i), fmt.Sprintf("project-%d", i))
		if _, err := repo.CreateProject(dto); err != nil {
			t.Fatalf("CreateProject %d: %v", i, err)
		}
	}

	projects, err := repo.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}

	slugSet := make(map[string]bool)
	for _, p := range projects {
		slugSet[p.Slug] = true
	}

	for i := 1; i <= 3; i++ {
		expected := fmt.Sprintf("project-%d", i)
		if !slugSet[expected] {
			t.Errorf("missing project %s", expected)
		}
	}
}

func TestFileProjectRepository_Project_ListProjects_SkipsNonDirEntries(t *testing.T) {
	repo, dir := newTestRepo(t)

	// Create a non-directory file in the root
	if err := os.WriteFile(filepath.Join(dir, "not-a-dir.txt"), []byte("ignore me"), common.DefaultFilePerm); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	projects, err := repo.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("expected 0 projects (skipped non-dir), got %d", len(projects))
	}
}

func TestFileProjectRepository_Project_ListProjects_SkipsBrokenProjects(t *testing.T) {
	repo, dir := newTestRepo(t)

	// Create a valid project
	dto := newTestDto(dir, "Valid", "valid")
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Create a broken project (directory without project.json)
	if err := os.MkdirAll(filepath.Join(dir, "broken"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	projects, err := repo.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project (skipped broken), got %d", len(projects))
	}

	if projects[0].Slug != "valid" {
		t.Errorf("expected 'valid', got %q", projects[0].Slug)
	}
}

func TestFileProjectRepository_Project_RenameProject_KeepsTheSlugAndTheDirectory(t *testing.T) {
	repo, dir := newTestRepo(t)

	dto := newTestDto(dir, "Old Name", "old-name")
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	taskFile := filepath.Join(dir, "old-name", "tasks", "task.md")
	if err := os.MkdirAll(filepath.Dir(taskFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(taskFile, []byte("a task"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := repo.RenameProject("old-name", "New Name"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "new-name")); !os.IsNotExist(err) {
		t.Errorf("expected no directory for the new name, got %v", err)
	}
	if _, err := os.Stat(taskFile); err != nil {
		t.Errorf("expected the tasks of the project to stay where they are: %v", err)
	}

	var proj project.Project
	if err := common.ReadJSON(filepath.Join(dir, "old-name", ProjectConfigFile), &proj); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	want := project.Project{Name: "New Name", Slug: "old-name", Location: dto.Location, CreatedAt: dto.CreatedAt}
	if !proj.CreatedAt.Equal(want.CreatedAt) || proj.Name != want.Name || proj.Slug != want.Slug || proj.Location != want.Location {
		t.Errorf("expected %+v, got %+v", want, proj)
	}
}

func TestFileProjectRepository_Project_RenameProject_NotFound(t *testing.T) {
	repo, _ := newTestRepo(t)

	if err := repo.RenameProject("nonexistent", "New Name"); err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}
