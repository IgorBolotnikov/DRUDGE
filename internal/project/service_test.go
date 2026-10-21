package project

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

// fakeProjectRepo holds projects in memory.
type fakeProjectRepo struct {
	projects  []*Project
	deleteErr error
}

func (repo *fakeProjectRepo) CreateProject(dto CreateProjectDto) (*Project, error) {
	created := &Project{Name: dto.Name, Slug: dto.Slug, CreatedAt: dto.CreatedAt}
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

func (repo *fakeProjectRepo) DeleteProject(slug string) (isRemoved bool, err error) {
	if repo.deleteErr != nil {
		return false, repo.deleteErr
	}
	for index, candidate := range repo.projects {
		if candidate.Slug == slug {
			repo.projects = append(repo.projects[:index], repo.projects[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// fakeProgress records the events a service reports.
type fakeProgress struct {
	events []any
}

func (progress *fakeProgress) Report(event any) {
	progress.events = append(progress.events, event)
}

// fakeLinker records the links it is handed. linkedSlug is the project the
// current directory is linked to, empty outside a project directory.
type fakeLinker struct {
	wasCalled    bool
	slug         string
	repositories []Repository
	envNames     []string
	overrides    common.Answers
	linkedSlug   string
}

func (linker *fakeLinker) LinkedSlug() (string, bool, error) {
	return linker.linkedSlug, linker.linkedSlug != "", nil
}

func (linker *fakeLinker) LinkDirectory(slug string, repositories []Repository, envNames []string, overrides common.Answers) error {
	linker.wasCalled = true
	linker.slug = slug
	linker.repositories = repositories
	linker.envNames = envNames
	linker.overrides = overrides
	return nil
}

// testLocalSchema holds the local config keys init asks about.
const testLocalSchema = `{
  "type": "object",
  "properties": {
    "maxConcurrentDrudgers": {"type": "integer", "minimum": 1, "default": 3},
    "remote": {
      "type": "object",
      "properties": {
        "provider": {"type": "string", "enum": ["github"]},
        "pullRequests": {
          "type": "object",
          "properties": {
            "isEnabled": {"type": "boolean", "default": false},
            "isDraft": {"type": "boolean", "default": false}
          }
        }
      }
    }
  }
}`

// fakeGlobalSettings resolves every key to its answer in answers, and to an
// empty answer when answers has none.
type fakeGlobalSettings struct {
	answers common.Answers
}

func (globals *fakeGlobalSettings) LocalSchema() []byte {
	return []byte(testLocalSchema)
}

func (globals *fakeGlobalSettings) ResolvedAnswers(keys []string) (common.Answers, error) {
	resolved := common.Answers{}
	for _, key := range keys {
		resolved[key] = globals.answers[key]
	}
	return resolved, nil
}

// defaultGlobalAnswers are the answers of a global config holding only the
// built-in defaults.
func defaultGlobalAnswers() common.Answers {
	return common.Answers{
		MaxConcurrentDrudgersKey:      "3",
		remote.PullRequestsEnabledKey: common.AnswerNo,
		remote.PullRequestsDraftKey:   common.AnswerNo,
	}
}

func TestProjectService_PrepareInit(t *testing.T) {
	tests := []struct {
		name          string
		existing      []*Project
		subdirs       []string
		roots         []string
		globalAnswers common.Answers
		// projectName is what the user names the project.
		projectName      string
		wantRepositories []Repository
		// wantDefaults maps the key of every question to the answer it starts
		// on.
		wantDefaults map[string]string
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects the init to be prepared.
		wantErrText string
	}{
		{
			name:             "a directory that is a repository",
			projectName:      "Test Project",
			roots:            []string{"."},
			globalAnswers:    defaultGlobalAnswers(),
			wantRepositories: []Repository{{Path: ".", Setup: []string{}, Copy: []string{}}},
			wantDefaults: map[string]string{
				MaxConcurrentDrudgersKey:      "3",
				remote.PullRequestsEnabledKey: common.AnswerNo,
				remote.PullRequestsDraftKey:   common.AnswerNo,
			},
		},
		{
			name:             "a directory holding repositories",
			projectName:      "My Cool App",
			subdirs:          []string{"api", "docs", "ui"},
			roots:            []string{"api", "ui"},
			globalAnswers:    defaultGlobalAnswers(),
			wantRepositories: []Repository{{Path: "api", Setup: []string{}, Copy: []string{}}, {Path: "ui", Setup: []string{}, Copy: []string{}}},
			wantDefaults: map[string]string{
				MaxConcurrentDrudgersKey:      "3",
				remote.PullRequestsEnabledKey: common.AnswerNo,
				remote.PullRequestsDraftKey:   common.AnswerNo,
			},
		},
		{
			name:        "a global config with its own values",
			projectName: "Test Project",
			roots:       []string{"."},
			globalAnswers: common.Answers{
				MaxConcurrentDrudgersKey:      "5",
				remote.PullRequestsEnabledKey: common.AnswerYes,
				remote.PullRequestsDraftKey:   common.AnswerYes,
				remote.ProviderKey:            string(remote.ProviderGitHub),
			},
			wantRepositories: []Repository{{Path: ".", Setup: []string{}, Copy: []string{}}},
			wantDefaults: map[string]string{
				MaxConcurrentDrudgersKey:      "5",
				remote.PullRequestsEnabledKey: common.AnswerYes,
				remote.PullRequestsDraftKey:   common.AnswerYes,
			},
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
			projectDir := testProjectDir
			repo := &fakeProjectRepo{projects: test.existing}
			linker := &fakeLinker{}
			progress := &fakeProgress{}
			globals := &fakeGlobalSettings{answers: test.globalAnswers}
			service := NewProjectService(repo, linker, nil, newFakeGit(projectDir, test.roots, nil), dirsOf(test.subdirs), globals, progress)

			prepared, err := service.PrepareInit(test.projectName, projectDir)

			if len(repo.projects) != len(test.existing) {
				t.Errorf("expected nothing written before the answers arrive, got %d projects", len(repo.projects))
			}
			if linker.wasCalled {
				t.Errorf("expected nothing linked before the answers arrive, got a link to %q", linker.slug)
			}
			if len(progress.events) != 0 {
				t.Errorf("expected nothing reported before the answers arrive, got %+v", progress.events)
			}
			if test.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErrText) {
					t.Fatalf("error = %v, want it to carry %q", err, test.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(prepared.Repositories, test.wantRepositories) {
				t.Errorf("repositories = %+v, want %+v", prepared.Repositories, test.wantRepositories)
			}
			gotDefaults := map[string]string{}
			for _, question := range prepared.Questions {
				gotDefaults[question.Key] = question.Default
			}
			if !reflect.DeepEqual(gotDefaults, test.wantDefaults) {
				t.Errorf("question defaults = %v, want %v", gotDefaults, test.wantDefaults)
			}
		})
	}
}

func TestProjectService_InitProject(t *testing.T) {
	tests := []struct {
		name          string
		globalAnswers common.Answers
		answers       common.Answers
		// wantOverrides are the keys written into the local config.
		wantOverrides common.Answers
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects the project to be made.
		wantErrText string
	}{
		{
			name:          "the defaults",
			globalAnswers: defaultGlobalAnswers(),
			answers:       common.Answers{},
			wantOverrides: common.Answers{},
		},
		{
			name:          "a concurrency that differs from the global config",
			globalAnswers: defaultGlobalAnswers(),
			answers:       common.Answers{MaxConcurrentDrudgersKey: "5"},
			wantOverrides: common.Answers{MaxConcurrentDrudgersKey: "5"},
		},
		{
			name:          "answers equal to the global config",
			globalAnswers: defaultGlobalAnswers(),
			answers:       common.Answers{MaxConcurrentDrudgersKey: "3", remote.PullRequestsEnabledKey: common.AnswerNo},
			wantOverrides: common.Answers{},
		},
		{
			name:          "pull requests turned on with no global provider",
			globalAnswers: defaultGlobalAnswers(),
			answers:       common.Answers{remote.PullRequestsEnabledKey: common.AnswerYes, remote.PullRequestsDraftKey: common.AnswerYes},
			wantOverrides: common.Answers{
				remote.PullRequestsEnabledKey: common.AnswerYes,
				remote.PullRequestsDraftKey:   common.AnswerYes,
				remote.ProviderKey:            string(remote.ProviderGitHub),
			},
		},
		{
			name: "pull requests turned on with a global provider",
			globalAnswers: common.Answers{
				MaxConcurrentDrudgersKey:      "3",
				remote.PullRequestsEnabledKey: common.AnswerNo,
				remote.PullRequestsDraftKey:   common.AnswerNo,
				remote.ProviderKey:            string(remote.ProviderGitHub),
			},
			answers:       common.Answers{remote.PullRequestsEnabledKey: common.AnswerYes},
			wantOverrides: common.Answers{remote.PullRequestsEnabledKey: common.AnswerYes},
		},
		{
			name: "pull requests turned off that the global config turns on",
			globalAnswers: common.Answers{
				MaxConcurrentDrudgersKey:      "3",
				remote.PullRequestsEnabledKey: common.AnswerYes,
				remote.PullRequestsDraftKey:   common.AnswerYes,
				remote.ProviderKey:            string(remote.ProviderGitHub),
			},
			answers:       common.Answers{remote.PullRequestsEnabledKey: common.AnswerNo},
			wantOverrides: common.Answers{remote.PullRequestsEnabledKey: common.AnswerNo},
		},
		{
			name:          "a concurrency below 1",
			globalAnswers: defaultGlobalAnswers(),
			answers:       common.Answers{MaxConcurrentDrudgersKey: "0"},
			wantErrText:   "it must be a positive number",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectDir := testProjectDir
			repo := &fakeProjectRepo{}
			linker := &fakeLinker{}
			progress := &fakeProgress{}
			globals := &fakeGlobalSettings{answers: test.globalAnswers}
			service := NewProjectService(repo, linker, nil, newFakeGit(projectDir, []string{"."}, nil), dirsOf(nil), globals, progress)
			prepared, err := service.PrepareInit("Test Project", projectDir)
			if err != nil {
				t.Fatalf("PrepareInit: %v", err)
			}

			created, err := service.InitProject(prepared, test.answers)

			if test.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErrText) {
					t.Fatalf("error = %v, want it to carry %q", err, test.wantErrText)
				}
				if len(repo.projects) != 0 || linker.wasCalled {
					t.Errorf("expected a refused answer to write nothing, got %d projects and a link to %q", len(repo.projects), linker.slug)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if created.Slug != "test-project" || linker.slug != created.Slug {
				t.Errorf("expected the directory to be linked to test-project, got project %q and link %q", created.Slug, linker.slug)
			}
			if !reflect.DeepEqual(linker.repositories, prepared.Repositories) {
				t.Errorf("expected the link to record %+v, got %+v", prepared.Repositories, linker.repositories)
			}
			if !reflect.DeepEqual(linker.overrides, test.wantOverrides) {
				t.Errorf("overrides = %v, want %v", linker.overrides, test.wantOverrides)
			}
			wantEvents := []any{ProjectCreated{Project: created}}
			if !reflect.DeepEqual(progress.events, wantEvents) {
				t.Errorf("events = %+v, want %+v", progress.events, wantEvents)
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
			service := NewProjectService(&fakeProjectRepo{projects: renamedProjects()}, nil, nil, nil, nil, nil, &fakeProgress{})

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
	service := NewProjectService(&fakeProjectRepo{projects: renamedProjects()}, nil, nil, nil, nil, nil, &fakeProgress{})

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
		// wantEvent is the event a successful rename reports.
		wantEvent ProjectRenamed
	}{
		{
			name:       "a project named by its slug",
			slugOrName: "blog",
			newName:    "Journal",
			wantNames:  map[string]string{"demo": "Shop", "blog": "Journal"},
			wantEvent:  ProjectRenamed{Slug: "blog", OldName: "Blog", NewName: "Journal"},
		},
		{
			name:       "a project named by its name",
			slugOrName: "Shop",
			newName:    "Store",
			wantNames:  map[string]string{"demo": "Store", "blog": "Blog"},
			wantEvent:  ProjectRenamed{Slug: "demo", OldName: "Shop", NewName: "Store"},
		},
		{
			name:       "a new name in the slug form of the old one",
			slugOrName: "Shop",
			newName:    "shop",
			wantNames:  map[string]string{"demo": "shop", "blog": "Blog"},
			wantEvent:  ProjectRenamed{Slug: "demo", OldName: "Shop", NewName: "shop"},
		},
		{
			name:       "a project given its slug back as its name",
			slugOrName: "Shop",
			newName:    "Demo",
			wantNames:  map[string]string{"demo": "Demo", "blog": "Blog"},
			wantEvent:  ProjectRenamed{Slug: "demo", OldName: "Shop", NewName: "Demo"},
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
			progress := &fakeProgress{}
			service := NewProjectService(repo, nil, nil, nil, nil, nil, progress)

			err := service.RenameProject(testCase.slugOrName, testCase.newName)

			var wantEvents []any
			if testCase.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Fatalf("expected the error to carry %q, got %v", testCase.wantErrText, err)
				}
				testCase.wantNames = map[string]string{"demo": "Shop", "blog": "Blog"}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				wantEvents = []any{testCase.wantEvent}
			}
			if !reflect.DeepEqual(progress.events, wantEvents) {
				t.Errorf("events = %+v, want %+v", progress.events, wantEvents)
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
	errDisk := errors.New("disk is full")

	cases := []struct {
		name      string
		project   *Project
		deleteErr error
		wantEvent any
		wantErr   error
		wantSlugs []string
	}{
		{
			name:      "a project",
			project:   &Project{Name: "Shop", Slug: "demo"},
			wantEvent: ProjectRemoved{Slug: "demo", Name: "Shop"},
			wantSlugs: []string{"blog"},
		},
		{
			name:      "a project that is already gone",
			project:   &Project{Name: "Wiki", Slug: "wiki"},
			wantEvent: ProjectAlreadyGone{Slug: "wiki"},
			wantSlugs: []string{"demo", "blog"},
		},
		{
			name:      "a repository that fails",
			project:   &Project{Name: "Shop", Slug: "demo"},
			deleteErr: errDisk,
			wantErr:   errDisk,
			wantSlugs: []string{"demo", "blog"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeProjectRepo{projects: renamedProjects(), deleteErr: testCase.deleteErr}
			progress := &fakeProgress{}
			service := NewProjectService(repo, nil, nil, nil, nil, nil, progress)

			err := service.DeleteProject(testCase.project)

			var wantEvents []any
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) {
					t.Fatalf("expected the error %v, got %v", testCase.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				wantEvents = []any{testCase.wantEvent}
			}
			if !reflect.DeepEqual(progress.events, wantEvents) {
				t.Errorf("events = %+v, want %+v", progress.events, wantEvents)
			}

			var gotSlugs []string
			for _, stored := range repo.projects {
				gotSlugs = append(gotSlugs, stored.Slug)
			}
			if !reflect.DeepEqual(gotSlugs, testCase.wantSlugs) {
				t.Errorf("expected the projects %v, got %v", testCase.wantSlugs, gotSlugs)
			}
		})
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
			service := NewProjectService(&fakeProjectRepo{projects: projects}, nil, nil, nil, nil, nil, &fakeProgress{})

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
