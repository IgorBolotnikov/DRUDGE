package project

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
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

func (repo *fakeProjectRepo) RenameProject(slug string, newName string) error {
	for _, candidate := range repo.projects {
		if candidate.Slug == slug {
			candidate.Name = newName
			return nil
		}
	}
	return errors.New("project not found")
}

func (repo *fakeProjectRepo) DeleteProject(slug string) error {
	return errors.New("DeleteProject should not be called")
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
			name:        "a name a renamed project goes by",
			existing:    []*Project{{Name: "Test Project", Slug: "demo"}},
			projectName: "test project",
			roots:       []string{"."},
			wantErrText: `project demo is already called "Test Project"`,
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

// renamedProjects are two projects, one of them renamed since it was made, so
// its name and its slug differ.
func renamedProjects() []*Project {
	return []*Project{
		{Name: "Shop", Slug: "demo"},
		{Name: "Blog", Slug: "blog"},
	}
}

func TestProjectService_LookupProject(t *testing.T) {
	cases := []struct {
		name       string
		slugOrName string
		wantSlug   string
	}{
		{name: "a slug", slugOrName: "demo", wantSlug: "demo"},
		{name: "the name of a renamed project", slugOrName: "Shop", wantSlug: "demo"},
		{name: "a name in its slug form", slugOrName: "shop", wantSlug: "demo"},
		{name: "a name that is also the slug", slugOrName: "Blog", wantSlug: "blog"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewProjectService(&fakeProjectRepo{projects: renamedProjects()}, nil, common.NewLogger(""))

			found, err := service.LookupProject(testCase.slugOrName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if found.Slug != testCase.wantSlug {
				t.Errorf("expected project %q, got %q", testCase.wantSlug, found.Slug)
			}
		})
	}
}

func TestProjectService_LookupProject_RefusesAnUnknownProject(t *testing.T) {
	service := NewProjectService(&fakeProjectRepo{projects: renamedProjects()}, nil, common.NewLogger(""))

	_, err := service.LookupProject("wiki")
	if err == nil || !strings.Contains(err.Error(), `project "wiki" not found`) {
		t.Fatalf("expected the lookup to name the missing project, got %v", err)
	}
}

func TestProjectService_RenameProject(t *testing.T) {
	cases := []struct {
		name       string
		slugOrName string
		newName    string

		// wantNames are the names of the projects by slug once the rename is
		// done.
		wantNames   map[string]string
		wantErrText string
	}{
		{
			name:       "a project named by its slug",
			slugOrName: "blog",
			newName:    "Journal",
			wantNames:  map[string]string{"demo": "Shop", "blog": "Journal"},
		},
		{
			name:       "a project named by its name",
			slugOrName: "Shop",
			newName:    "Store",
			wantNames:  map[string]string{"demo": "Store", "blog": "Blog"},
		},
		{
			name:       "a new name in the slug form of the old one",
			slugOrName: "Shop",
			newName:    "shop",
			wantNames:  map[string]string{"demo": "shop", "blog": "Blog"},
		},
		{
			name:       "a project given its slug back as its name",
			slugOrName: "Shop",
			newName:    "Demo",
			wantNames:  map[string]string{"demo": "Demo", "blog": "Blog"},
		},
		{
			name:        "a name another project goes by",
			slugOrName:  "blog",
			newName:     "shop",
			wantErrText: `project demo is already called "Shop"`,
		},
		{
			name:        "the slug of another project",
			slugOrName:  "Shop",
			newName:     "Blog",
			wantErrText: "project blog already exists",
		},
		{
			name:        "an unknown project",
			slugOrName:  "wiki",
			newName:     "Notes",
			wantErrText: `project "wiki" not found`,
		},
		{
			name:        "an empty new name",
			slugOrName:  "blog",
			newName:     "",
			wantErrText: "new project name is required",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeProjectRepo{projects: renamedProjects()}
			service := NewProjectService(repo, nil, common.NewLogger(""))

			err := service.RenameProject(testCase.slugOrName, testCase.newName)

			if testCase.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Fatalf("expected the error to carry %q, got %v", testCase.wantErrText, err)
				}
				testCase.wantNames = map[string]string{"demo": "Shop", "blog": "Blog"}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotNames := map[string]string{}
			for _, stored := range repo.projects {
				gotNames[stored.Slug] = stored.Name
			}
			if !reflect.DeepEqual(gotNames, testCase.wantNames) {
				t.Errorf("expected the projects %v, got %v", testCase.wantNames, gotNames)
			}
		})
	}
}
