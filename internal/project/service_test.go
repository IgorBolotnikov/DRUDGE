package project

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
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

var errFakeProjectNotFound = errors.New("project not found")

func (repo *fakeProjectRepo) DeleteProject(slug string) error {
	for index, candidate := range repo.projects {
		if candidate.Slug == slug {
			repo.projects = append(repo.projects[:index], repo.projects[index+1:]...)
			return nil
		}
	}
	return errFakeProjectNotFound
}

// fakeLinker records the links it is handed.
type fakeLinker struct {
	wasCalled    bool
	slug         string
	repositories []Repository
}

func (linker *fakeLinker) LinkDirectory(slug string, repositories []Repository) error {
	linker.wasCalled = true
	linker.slug = slug
	linker.repositories = repositories
	return nil
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
		wantRepositories []Repository
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects the project to be made.
		wantErrText string
	}{
		{
			name:             "a directory that is a repository",
			projectName:      "Test Project",
			roots:            []string{"."},
			wantSlug:         "test-project",
			wantRepositories: []Repository{{Path: "."}},
		},
		{
			name:             "a directory holding repositories",
			projectName:      "My Cool App",
			subdirs:          []string{"api", "docs", "ui"},
			roots:            []string{"api", "ui"},
			wantSlug:         "my-cool-app",
			wantRepositories: []Repository{{Path: "api"}, {Path: "ui"}},
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

			repo := &fakeProjectRepo{projects: test.existing}
			linker := &fakeLinker{}
			service := NewProjectService(repo, linker, newFakeGit(projectDir, test.roots, nil), common.NewLogger(""))

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
				if linker.wasCalled {
					t.Errorf("expected a refused project to be left unlinked, got a link to %q", linker.slug)
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
			if linker.slug != test.wantSlug {
				t.Errorf("expected the directory to be linked to %q, got %q", test.wantSlug, linker.slug)
			}
			if !reflect.DeepEqual(linker.repositories, test.wantRepositories) {
				t.Errorf("expected the link to record %+v, got %+v", test.wantRepositories, linker.repositories)
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
			service := NewProjectService(&fakeProjectRepo{projects: renamedProjects()}, nil, nil, common.NewLogger(""))

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
	service := NewProjectService(&fakeProjectRepo{projects: renamedProjects()}, nil, nil, common.NewLogger(""))

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
			service := NewProjectService(repo, nil, nil, common.NewLogger(""))

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

func TestProjectService_DeleteProject(t *testing.T) {
	repo := &fakeProjectRepo{projects: renamedProjects()}
	service := NewProjectService(repo, nil, nil, common.NewLogger(""))

	if err := service.DeleteProject("demo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	remaining, err := repo.ListProjects()
	if err != nil {
		t.Fatalf("could not list projects: %v", err)
	}
	for _, candidate := range remaining {
		if candidate.Slug == "demo" {
			t.Errorf("expected project demo to be gone, got %+v", remaining)
		}
	}
	if len(remaining) != 1 {
		t.Errorf("expected the other project to stay, got %+v", remaining)
	}
}

func TestProjectService_DeleteProject_ReturnsTheErrorOfTheRepository(t *testing.T) {
	service := NewProjectService(&fakeProjectRepo{projects: renamedProjects()}, nil, nil, common.NewLogger(""))

	err := service.DeleteProject("wiki")
	if !errors.Is(err, errFakeProjectNotFound) {
		t.Fatalf("expected the error of the repository, got %v", err)
	}
}

func TestProjectService_ListProjects(t *testing.T) {
	projects := []*Project{{Slug: "api"}, {Slug: "docs"}, {Slug: "ui"}}

	tests := []struct {
		name      string
		page      int
		size      int
		wantSlugs []string
		// wantErr is the whole refusal. A case without it expects a page.
		wantErr string
	}{
		{name: "a page", page: 2, size: 2, wantSlugs: []string{"ui"}},
		{name: "a page past the last one names the projects", page: 3, size: 2, wantErr: "page 3 does not exist, there are 2 pages of projects"},
		{name: "a page below 1", page: 0, size: 2, wantErr: "page must be 1 or more, got 0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewProjectService(&fakeProjectRepo{projects: projects}, nil, nil, common.NewLogger(""))

			listed, err := service.ListProjects(test.page, test.size)
			if test.wantErr != "" {
				if err == nil || err.Error() != test.wantErr {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			slugs := make([]string, 0, len(listed.Items))
			for _, listedProject := range listed.Items {
				slugs = append(slugs, listedProject.Slug)
			}
			if !reflect.DeepEqual(slugs, test.wantSlugs) {
				t.Errorf("slugs = %v, want %v", slugs, test.wantSlugs)
			}
		})
	}
}
