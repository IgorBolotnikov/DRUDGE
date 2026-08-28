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
		CreatedAt: time.Now(),
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
	if err := os.WriteFile(filepath.Join(dir, "not-a-dir.txt"), []byte("ignore me"), 0o644); err != nil {
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

func TestFileProjectRepository_Project_LookupProject_BySlug(t *testing.T) {
	repo, dir := newTestRepo(t)

	dto := newTestDto(dir, "Lookup Test", "lookup-test")
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	proj, err := repo.LookupProject("lookup-test")
	if err != nil {
		t.Fatalf("LookupProject: %v", err)
	}

	if proj.Slug != "lookup-test" || proj.Name != "Lookup Test" {
		t.Errorf("unexpected project: %+v", proj)
	}
}

func TestFileProjectRepository_Project_LookupProject_ByLowerName(t *testing.T) {
	repo, dir := newTestRepo(t)

	dto := newTestDto(dir, "Lookup Test", "lookup-test")
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	proj, err := repo.LookupProject("lookup test")
	if err != nil {
		t.Fatalf("LookupProject: %v", err)
	}

	if proj.Slug != "lookup-test" {
		t.Errorf("expected 'lookup-test', got %q", proj.Slug)
	}
}

func TestFileProjectRepository_Project_LookupProject_ByMixedCaseName(t *testing.T) {
	repo, dir := newTestRepo(t)

	dto := newTestDto(dir, "My Awesome Project", "my-awesome-project")
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	proj, err := repo.LookupProject("my awesome project")
	if err != nil {
		t.Fatalf("LookupProject: %v", err)
	}

	if proj.Name != "My Awesome Project" {
		t.Errorf("expected 'My Awesome Project', got %q", proj.Name)
	}
}

func TestFileProjectRepository_Project_LookupProject_NotFound(t *testing.T) {
	repo, _ := newTestRepo(t)

	_, err := repo.LookupProject("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestFileProjectRepository_Project_RenameProject_Collision(t *testing.T) {
	repo, dir := newTestRepo(t)

	// Create two projects
	dto1 := newTestDto(dir, "Alpha", "alpha")
	dto2 := newTestDto(dir, "Beta", "beta")
	if _, err := repo.CreateProject(dto1); err != nil {
		t.Fatalf("CreateProject alpha: %v", err)
	}
	if _, err := repo.CreateProject(dto2); err != nil {
		t.Fatalf("CreateProject beta: %v", err)
	}

	// Try to rename alpha to beta — should fail
	err := repo.RenameProject("alpha", "Beta")
	if err == nil {
		t.Fatal("expected error for collision")
	}
}

func TestFileProjectRepository_Project_RenameProject_SameSlug(t *testing.T) {
	repo, dir := newTestRepo(t)

	dto := newTestDto(dir, "old-name", "old-name")
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := repo.RenameProject("old-name", "old-name"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	// Read updated project.json
	var proj project.Project
	if err := common.ReadJSON(filepath.Join(dir, "old-name", ProjectConfigFile), &proj); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}

	if proj.Name != "old-name" {
		t.Errorf("expected 'old-name', got %q", proj.Name)
	}
	if proj.Slug != "old-name" {
		t.Errorf("expected slug 'old-name' (unchanged), got %q", proj.Slug)
	}
}

func TestFileProjectRepository_Project_RenameProject_DirMoved(t *testing.T) {
	repo, dir := newTestRepo(t)

	dto := newTestDto(dir, "Old Name", "old-name-123")
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := repo.RenameProject("old-name-123", "New Name"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	// Old dir should be gone
	_, err := os.Stat(filepath.Join(dir, "old-name-123"))
	if !os.IsNotExist(err) {
		t.Error("old directory should be removed")
	}

	// New dir should exist with updated data
	_, err = os.Stat(filepath.Join(dir, "new-name"))
	if err != nil {
		t.Fatal("new directory should exist")
	}

	var proj project.Project
	if err := common.ReadJSON(filepath.Join(dir, "new-name", ProjectConfigFile), &proj); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}

	if proj.Slug != "new-name" {
		t.Errorf("expected slug 'new-name', got %q", proj.Slug)
	}
	if proj.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %q", proj.Name)
	}
}
