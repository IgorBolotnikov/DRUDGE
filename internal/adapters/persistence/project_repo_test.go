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

func TestFileProjectRepository_Project_DeleteProject_DoesNotExist(t *testing.T) {
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	err := repo.DeleteProject("nonexistent")
	if err != nil {
		t.Errorf("expected no error for nonexistent project: %v", err)
	}
}

func TestFileProjectRepository_Project_DeleteProject_RemovesDir(t *testing.T) {
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	dto := project.CreateProjectDto{
		Name:     "Delete Test",
		Slug:     "delete-test",
		Location: filepath.Join(dir, "delete-test"),
		CreatedAt: time.Now(),
	}
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	projects, err := repo.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestFileProjectRepository_Project_ListProjects_Single(t *testing.T) {
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	dto := project.CreateProjectDto{
		Name:     "One Project",
		Slug:     "one-project",
		Location: filepath.Join(dir, "one-project"),
		CreatedAt: time.Now(),
	}
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	for i := 1; i <= 3; i++ {
		dto := project.CreateProjectDto{
			Name:     fmt.Sprintf("Project %d", i),
			Slug:     fmt.Sprintf("project-%d", i),
			Location: filepath.Join(dir, fmt.Sprintf("project-%d", i)),
			CreatedAt: time.Now(),
		}
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	// Create a valid project
	dto := project.CreateProjectDto{
		Name:     "Valid",
		Slug:     "valid",
		Location: filepath.Join(dir, "valid"),
		CreatedAt: time.Now(),
	}
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	dto := project.CreateProjectDto{
		Name:     "Lookup Test",
		Slug:     "lookup-test",
		Location: filepath.Join(dir, "lookup-test"),
		CreatedAt: time.Now(),
	}
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	dto := project.CreateProjectDto{
		Name:     "Lookup Test",
		Slug:     "lookup-test",
		Location: filepath.Join(dir, "lookup-test"),
		CreatedAt: time.Now(),
	}
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	dto := project.CreateProjectDto{
		Name:     "My Awesome Project",
		Slug:     "my-awesome-project",
		Location: filepath.Join(dir, "my-awesome-project"),
		CreatedAt: time.Now(),
	}
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	_, err := repo.LookupProject("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestFileProjectRepository_Project_RenameProject_Collision(t *testing.T) {
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	// Create two projects
	dto1 := project.CreateProjectDto{
		Name:     "Alpha",
		Slug:     "alpha",
		Location: filepath.Join(dir, "alpha"),
		CreatedAt: time.Now(),
	}
	dto2 := project.CreateProjectDto{
		Name:     "Beta",
		Slug:     "beta",
		Location: filepath.Join(dir, "beta"),
		CreatedAt: time.Now(),
	}
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	dto := project.CreateProjectDto{
		Name:    "old-name",
		Slug:    "old-name",
		Location: filepath.Join(dir, "old-name"),
		CreatedAt: time.Now(),
	}
	if _, err := repo.CreateProject(dto); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := repo.RenameProject("old-name", "old-name"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	// Read updated project.json
	var proj project.Project
	if err := common.ReadJSON(filepath.Join(dir, "old-name", "project.json"), &proj); err != nil {
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
	dir := t.TempDir()
	repo := NewFileProjectRepository(dir)

	dto := project.CreateProjectDto{
		Name:    "Old Name",
		Slug:    "old-name-123",
		Location: filepath.Join(dir, "old-name-123"),
		CreatedAt: time.Now(),
	}
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
	if err := common.ReadJSON(filepath.Join(dir, "new-name", "project.json"), &proj); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}

	if proj.Slug != "new-name" {
		t.Errorf("expected slug 'new-name', got %q", proj.Slug)
	}
	if proj.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %q", proj.Name)
	}
}
