package project

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"drudge/internal/common"
	"drudge/internal/config"
)

// fakeProjectRepo holds projects in memory.
type fakeProjectRepo struct {
	projects []*Project
}

func (repo *fakeProjectRepo) CreateProject(dto CreateProjectDto) (*Project, error) {
	created := &Project{Name: dto.Name, Slug: dto.Slug, Location: dto.Location, CreatedAt: dto.CreatedAt}
	repo.projects = append(repo.projects, created)
	return created, nil
}

func (repo *fakeProjectRepo) ListProjects() ([]*Project, error) {
	return repo.projects, nil
}

func (repo *fakeProjectRepo) LookupProject(slug string) (*Project, error) {
	for _, candidate := range repo.projects {
		if candidate.Slug == slug {
			return candidate, nil
		}
	}
	return nil, errors.New("project not found")
}

func (repo *fakeProjectRepo) RenameProject(slug string, newName string) error {
	return errors.New("RenameProject should not be called")
}

func (repo *fakeProjectRepo) DeleteProject(slug string) error {
	return errors.New("DeleteProject should not be called")
}

func (repo *fakeProjectRepo) ProjectExists(slug string) bool {
	_, err := repo.LookupProject(slug)
	return err == nil
}

func TestProjectService_InitProject(t *testing.T) {
	tests := []struct {
		name     string
		existing []*Project
		subdirs  []string
		roots    []string
		// projectName is what the user names the project.
		projectName      string
		wantSlug         string
		wantRepositories []config.Repository
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects the project to be made.
		wantErrText string
	}{
		{
			name:             "a directory that is a repository",
			projectName:      "Test Project",
			roots:            []string{"."},
			wantSlug:         "test-project",
			wantRepositories: []config.Repository{{Path: "."}},
		},
		{
			name:             "a directory holding repositories",
			projectName:      "My Cool App",
			subdirs:          []string{"api", "docs", "ui"},
			roots:            []string{"api", "ui"},
			wantSlug:         "my-cool-app",
			wantRepositories: []config.Repository{{Path: "api"}, {Path: "ui"}},
		},
		{
			name:        "a directory holding no repository",
			projectName: "Test Project",
			subdirs:     []string{"docs"},
			wantErrText: "is not a git repository and holds none",
		},
		{
			name:        "a project that already exists",
			existing:    []*Project{{Name: "Test Project", Slug: "test-project"}},
			projectName: "Test Project",
			roots:       []string{"."},
			wantErrText: "project test-project already exists",
		},
		{
			name:        "no name",
			roots:       []string{"."},
			wantErrText: "project name is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			projectDir := makeProjectDir(t, test.subdirs)
			t.Chdir(projectDir)

			repo := &fakeProjectRepo{projects: test.existing}
			service := NewProjectService(repo, newFakeGit(projectDir, test.roots, nil), common.NewLogger(""))

			created, repositories, err := service.InitProject(test.projectName, projectDir)

			if test.wantErrText != "" {
				if err == nil {
					t.Fatal("expected the project to be refused")
				}
				if !strings.Contains(err.Error(), test.wantErrText) {
					t.Errorf("expected the error to carry %q, got %q", test.wantErrText, err)
				}
				if len(repo.projects) != len(test.existing) {
					t.Errorf("expected a refused project to be left unwritten, got %d projects", len(repo.projects))
				}
				if _, err := os.Stat(common.LocalConfigPath()); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("expected no local config file, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if created.Slug != test.wantSlug {
				t.Errorf("expected the slug %q, got %q", test.wantSlug, created.Slug)
			}
			if !reflect.DeepEqual(repositories, test.wantRepositories) {
				t.Errorf("expected the repositories %+v, got %+v", test.wantRepositories, repositories)
			}

			saved, err := config.LoadLocal()
			if err != nil {
				t.Fatalf("could not read the local config: %v", err)
			}
			if saved.ProjectSlug != test.wantSlug {
				t.Errorf("expected the local config to link %q, got %q", test.wantSlug, saved.ProjectSlug)
			}
			if !reflect.DeepEqual(saved.Repositories, test.wantRepositories) {
				t.Errorf("expected the local config to record %+v, got %+v", test.wantRepositories, saved.Repositories)
			}
		})
	}
}
