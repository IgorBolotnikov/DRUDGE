package persistence

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

const testDirPerm = 0o755

// testProject is a project a test stores before it runs the method under test.
type testProject struct {
	name string
	slug string
}

// newTestRepo returns a repository over a projects directory holding the
// given projects, and the path of that directory.
func newTestRepo(t *testing.T, projects ...testProject) (*FileProjectRepository, string) {
	t.Helper()
	projectsDir := t.TempDir()
	repo := NewFileProjectRepository(projectsDir)

	for _, stored := range projects {
		dto := project.CreateProjectDto{
			Name:      stored.name,
			Slug:      stored.slug,
			Location:  filepath.Join(projectsDir, stored.slug),
			CreatedAt: time.Now().UTC(),
		}
		if _, err := repo.CreateProject(dto); err != nil {
			t.Fatalf("could not create project %s: %v", stored.slug, err)
		}
	}
	return repo, projectsDir
}

func TestFileProjectRepository_DeleteProject(t *testing.T) {
	cases := []struct {
		name     string
		projects []testProject
		slug     string
		// wantSlugs are the project directories left once the delete is done,
		// in name order.
		wantSlugs []string
	}{
		{
			name:      "a project",
			projects:  []testProject{{name: "Shop", slug: "shop"}, {name: "Blog", slug: "blog"}},
			slug:      "shop",
			wantSlugs: []string{"blog"},
		},
		{
			name:      "a project that does not exist",
			projects:  []testProject{{name: "Blog", slug: "blog"}},
			slug:      "shop",
			wantSlugs: []string{"blog"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, projectsDir := newTestRepo(t, testCase.projects...)

			if err := repo.DeleteProject(testCase.slug); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			entries, err := os.ReadDir(projectsDir)
			if err != nil {
				t.Fatal(err)
			}
			var gotSlugs []string
			for _, entry := range entries {
				gotSlugs = append(gotSlugs, entry.Name())
			}
			if !slices.Equal(gotSlugs, testCase.wantSlugs) {
				t.Errorf("expected the project directories %v, got %v", testCase.wantSlugs, gotSlugs)
			}
		})
	}
}

func TestFileProjectRepository_ListProjects(t *testing.T) {
	cases := []struct {
		name     string
		projects []testProject
		// strayFiles are files in the projects directory that are not projects.
		strayFiles []string
		// brokenDirs are directories in the projects directory without a
		// project file.
		brokenDirs []string
		// hasNoProjectsDir removes the projects directory before the listing.
		hasNoProjectsDir bool

		// wantNames are the names of the listed projects by slug.
		wantNames map[string]string
	}{
		{
			name:      "no projects",
			wantNames: map[string]string{},
		},
		{
			name:             "no projects directory",
			hasNoProjectsDir: true,
			wantNames:        map[string]string{},
		},
		{
			name:      "one project",
			projects:  []testProject{{name: "One Project", slug: "one-project"}},
			wantNames: map[string]string{"one-project": "One Project"},
		},
		{
			name: "several projects",
			projects: []testProject{
				{name: "Project 1", slug: "project-1"},
				{name: "Project 2", slug: "project-2"},
				{name: "Project 3", slug: "project-3"},
			},
			wantNames: map[string]string{"project-1": "Project 1", "project-2": "Project 2", "project-3": "Project 3"},
		},
		{
			name:       "a file that is not a project",
			strayFiles: []string{"not-a-dir.txt"},
			wantNames:  map[string]string{},
		},
		{
			name:       "a directory without a project file",
			projects:   []testProject{{name: "Valid", slug: "valid"}},
			brokenDirs: []string{"broken"},
			wantNames:  map[string]string{"valid": "Valid"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, projectsDir := newTestRepo(t, testCase.projects...)
			for _, file := range testCase.strayFiles {
				if err := os.WriteFile(filepath.Join(projectsDir, file), []byte("ignore me"), common.DefaultFilePerm); err != nil {
					t.Fatal(err)
				}
			}
			for _, dir := range testCase.brokenDirs {
				if err := os.MkdirAll(filepath.Join(projectsDir, dir), testDirPerm); err != nil {
					t.Fatal(err)
				}
			}
			if testCase.hasNoProjectsDir {
				if err := os.RemoveAll(projectsDir); err != nil {
					t.Fatal(err)
				}
			}

			projects, err := repo.ListProjects()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotNames := map[string]string{}
			for _, listed := range projects {
				gotNames[listed.Slug] = listed.Name
			}
			if !maps.Equal(gotNames, testCase.wantNames) {
				t.Errorf("expected the projects %v, got %v", testCase.wantNames, gotNames)
			}
		})
	}
}

func TestFileProjectRepository_RenameProject(t *testing.T) {
	const (
		storedName = "Old Name"
		storedSlug = "old-name"
	)

	cases := []struct {
		name    string
		slug    string
		newName string

		wantErr bool
	}{
		{name: "a new name", slug: storedSlug, newName: "New Name"},
		{name: "the same name", slug: storedSlug, newName: storedName},
		{name: "a project that does not exist", slug: "nonexistent", newName: "New Name", wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, projectsDir := newTestRepo(t, testProject{name: storedName, slug: storedSlug})
			projectDir := filepath.Join(projectsDir, storedSlug)
			taskFile := filepath.Join(projectDir, "tasks", "task.md")
			if err := os.MkdirAll(filepath.Dir(taskFile), testDirPerm); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(taskFile, []byte("a task"), common.DefaultFilePerm); err != nil {
				t.Fatal(err)
			}

			err := repo.RenameProject(testCase.slug, testCase.newName)

			wantName := testCase.newName
			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				wantName = storedName
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if _, err := os.Stat(taskFile); err != nil {
				t.Errorf("expected the tasks of the project to stay where they are: %v", err)
			}
			if newSlug := common.SlugFrom(testCase.newName); newSlug != storedSlug {
				if _, err := os.Stat(filepath.Join(projectsDir, newSlug)); !os.IsNotExist(err) {
					t.Errorf("expected no directory for the new name, got %v", err)
				}
			}

			var got project.Project
			if err := common.ReadJSON(filepath.Join(projectDir, ProjectConfigFile), &got); err != nil {
				t.Fatalf("could not read the project file: %v", err)
			}
			if got.Name != wantName || got.Slug != storedSlug || got.Location != projectDir {
				t.Errorf("expected name %q, slug %q and location %q, got %+v", wantName, storedSlug, projectDir, got)
			}
		})
	}
}
