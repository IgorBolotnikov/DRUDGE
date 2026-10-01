package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

func TestRunProject_PrintsHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "the help flag", args: []string{ProjectCmd.Name, "--help"}},
		{name: "no subcommand", args: []string{ProjectCmd.Name}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var err error
			output := captureOutput(func() { err = NewRoot("v1.2.3").Execute(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(output, "usage: drg project <subcommand>\n") {
				t.Errorf("expected the help of the project command, got:\n%s", output)
			}
			for _, sub := range ProjectCmd.Subcommands {
				if !strings.Contains(output, "  "+sub.Name+" ") {
					t.Errorf("expected %q in the help, got:\n%s", sub.Name, output)
				}
			}
		})
	}
}

func TestRunProject_RefusesAnUnknownSubcommand(t *testing.T) {
	err := NewRoot("v1.2.3").Execute([]string{ProjectCmd.Name, "move"})

	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), `unknown subcommand "move"`) {
		t.Errorf("expected the error to name the unknown subcommand, got %q", err)
	}
}

func TestProjectInit(t *testing.T) {
	cases := []struct {
		name string
		// repositories are created in the project directory, each mapped to
		// the branch origin/HEAD points at. An empty branch leaves origin/HEAD
		// unset.
		repositories map[string]string
		wantStdout   string
		wantStderr   string
		wantErr      string
	}{
		{
			name:         "a project with one resolved and one unresolved repository",
			repositories: map[string]string{"a": "", "b": "main"},
			wantStdout: "Initializing project demo in .drudge\n" +
				"✓ Initialized project demo\n" +
				"\n" +
				"Repositories (2):\n" +
				"  REPOSITORY                      DEFAULT BRANCH\n" +
				"  ------------------------------  --------------\n" +
				"  a                               unresolved\n" +
				"  b                               main\n",
			wantStderr: "  ! could not work out the default branch of a, run `git remote set-head origin -a` in it, " +
				"or set \"defaultBranch\" for it in the local config\n",
		},
		{
			name:       "a directory that holds no repository",
			wantStdout: "Initializing project demo in .drudge\n",
			wantErr:    "is not a git repository and holds none",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			projectDir := t.TempDir()
			t.Chdir(projectDir)
			for path, branch := range testCase.repositories {
				dir := filepath.Join(projectDir, path)
				runGit(t, projectDir, "init", "-q", dir)
				if branch != "" {
					runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/"+branch)
				}
			}

			var err error
			var stdout string
			stderr := captureStderr(func() {
				stdout = captureOutput(func() { err = NewRoot("v1.2.3").Execute([]string{ProjectCmd.Name, "init", "demo"}) })
			})

			if testCase.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("error = %v, want it to hold %q", err, testCase.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stdout != testCase.wantStdout {
				t.Errorf("stdout:\n%q\nwant:\n%q", stdout, testCase.wantStdout)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr:\n%q\nwant:\n%q", stderr, testCase.wantStderr)
			}
		})
	}
}

// runGit runs git in dir with the git variables of the environment cleared,
// so a git hook running the tests does not point it at another repository.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, gitVariablePrefix) {
			command.Env = append(command.Env, variable)
		}
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, output)
	}
}

const gitVariablePrefix = "GIT_"

func TestRepositoryLines(t *testing.T) {
	cases := []struct {
		name     string
		resolved []project.ResolvedRepository
		// wantLines are substrings the listing prints, in the order given.
		wantLines []string
	}{
		{
			name: "a repository that is the project directory",
			resolved: []project.ResolvedRepository{
				{Repository: project.Repository{Path: "."}, DefaultBranch: "main"},
			},
			wantLines: []string{"Repositories (1):", "REPOSITORY", "DEFAULT BRANCH", ".", "main"},
		},
		{
			name: "a repository whose default branch does not resolve",
			resolved: []project.ResolvedRepository{
				{Repository: project.Repository{Path: "api"}, DefaultBranch: "trunk"},
				{Repository: project.Repository{Path: "ui"}, Problem: errors.New("no origin/HEAD is set")},
			},
			wantLines: []string{"Repositories (2):", "api", "trunk", "ui", unresolvedBranch},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			output := strings.Join(repositoryLines(testCase.resolved), "\n")

			rest := output
			for _, want := range testCase.wantLines {
				index := strings.Index(rest, want)
				if index < 0 {
					t.Fatalf("expected the listing to hold %q after the lines before it, got:\n%s", want, output)
				}
				rest = rest[index+len(want):]
			}
		})
	}
}

func TestProjectList(t *testing.T) {
	const header = "  SLUG                  NAME\n" +
		"  --------------------  ----\n"

	cases := []struct {
		name     string
		projects []string
		args     []string
		// globalConfig is written to the global config file when set.
		globalConfig string
		env          map[string]string
		want         string
		wantErr      string
	}{
		{
			name:     "a listing that fits on one page prints no footer",
			projects: []string{"alpha", "bravo"},
			want:     "Projects (2):\n" + header + "  alpha                 alpha\n  bravo                 bravo\n",
		},
		{
			name:     "a page before the last one hints at the next one",
			projects: []string{"alpha", "bravo", "charlie", "delta", "echo"},
			args:     []string{"--page-size", "2"},
			want: "Projects (5):\n" + header + "  alpha                 alpha\n  bravo                 bravo\n" +
				"Page 1 of 3, see the next one with --page 2\n",
		},
		{
			name:     "the last page prints a footer without a hint",
			projects: []string{"alpha", "bravo", "charlie", "delta", "echo"},
			args:     []string{"-p", "3", "--page-size", "2"},
			want:     "Projects (5):\n" + header + "  echo                  echo\n" + "Page 3 of 3\n",
		},
		{
			name:         "the global config sets the page size",
			projects:     []string{"alpha", "bravo", "charlie"},
			args:         []string{"--page", "2"},
			globalConfig: `{"project": {"pageSize": 2}}`,
			want:         "Projects (3):\n" + header + "  charlie               charlie\n" + "Page 2 of 2\n",
		},
		{
			name:         "a page size of 0 on the command line turns paging off",
			projects:     []string{"alpha", "bravo", "charlie"},
			args:         []string{"--page-size", "0"},
			globalConfig: `{"project": {"pageSize": 2}}`,
			want: "Projects (3):\n" + header +
				"  alpha                 alpha\n  bravo                 bravo\n  charlie               charlie\n",
		},
		{
			name:     "the footer prints in the muted color of the system theme",
			projects: []string{"alpha", "bravo"},
			args:     []string{"--page", "2", "--page-size", "1"},
			env:      map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			want: "Projects (2):\n" + header + "  bravo                 bravo\n" +
				"\x1b[2mPage 2 of 2\x1b[0m\n",
		},
		{
			name: "no projects",
			want: "· No projects yet, run drg project init <name> in a project directory to create one\n",
		},
		{
			name:    "a page of no projects past the first one",
			args:    []string{"--page", "2"},
			wantErr: "page 2 does not exist, there is 1 page of projects",
		},
		{
			name:     "a page past the last one",
			projects: []string{"alpha", "bravo", "charlie"},
			args:     []string{"--page", "5", "--page-size", "1"},
			wantErr:  "page 5 does not exist, there are 3 pages of projects",
		},
		{
			name:    "a page below 1",
			args:    []string{"--page", "0"},
			wantErr: "page must be 1 or more, got 0",
		},
		{
			name:    "a negative page size",
			args:    []string{"--page-size", "-1"},
			wantErr: "--page-size must be 0 or more, got -1",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("NO_COLOR", "1")
			for name, value := range testCase.env {
				t.Setenv(name, value)
			}
			if testCase.globalConfig != "" {
				if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(common.GlobalConfigPath(home), []byte(testCase.globalConfig), common.DefaultFilePerm); err != nil {
					t.Fatal(err)
				}
			}
			service := project.NewProjectService(persistence.NewFileProjectRepository(""), nil, nil, newTestCLIProgress())
			captureOutput(func() {
				for _, name := range testCase.projects {
					if _, err := service.CreateProject(name); err != nil {
						t.Fatal(err)
					}
				}
			})

			var err error
			args := append([]string{ProjectCmd.Name, "list"}, testCase.args...)
			output := captureOutput(func() { err = NewRoot("v1.2.3").Execute(args) })

			if testCase.wantErr != "" {
				if err == nil || err.Error() != testCase.wantErr {
					t.Fatalf("error = %v, want %q", err, testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output != testCase.want {
				t.Errorf("expected:\n%q\ngot:\n%q", testCase.want, output)
			}
		})
	}
}

func TestProjectList_WarnsAboutTheThemeOnce(t *testing.T) {
	const listing = "Projects (2):\n" +
		"  SLUG                  NAME\n" +
		"  --------------------  ----\n" +
		"  alpha                 alpha\n" +
		"Page 1 of 2, see the next one with --page 2\n"

	cases := []struct {
		name       string
		themeFile  string
		wantStdout string
		wantStderr string
	}{
		{
			name:       "a valid theme warns nothing",
			themeFile:  `{"theme": "nord", "overrides": {"error": "#ff0000"}}`,
			wantStdout: listing,
			wantStderr: "",
		},
		{
			name:       "a bad override warns once",
			themeFile:  `{"theme": "nord", "overrides": {"error": "#zzz"}}`,
			wantStdout: "\n" + listing,
			wantStderr: "! theme.json: \"#zzz\" is not a color for role error, using the theme's own\n",
		},
		{
			name:       "every bad override warns once",
			themeFile:  `{"theme": "nord", "overrides": {"warning": "yellow", "error": "#zzz"}}`,
			wantStdout: "\n" + listing,
			wantStderr: "! theme.json: \"#zzz\" is not a color for role error, using the theme's own\n" +
				"! theme.json: \"yellow\" is not a color for role warning, using the theme's own\n",
		},
		{
			name:       "a theme that fails to load warns once",
			themeFile:  `{"theme": "no-such-theme"}`,
			wantStdout: "\n" + listing,
			wantStderr: "! cannot load the theme, using the default one: unknown theme \"no-such-theme\"\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("NO_COLOR", "1")
			if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(common.ThemeConfigPath(home), []byte(testCase.themeFile), common.DefaultFilePerm); err != nil {
				t.Fatal(err)
			}
			service := project.NewProjectService(persistence.NewFileProjectRepository(""), nil, nil, newTestCLIProgress())
			captureOutput(func() {
				for _, name := range []string{"alpha", "bravo"} {
					if _, err := service.CreateProject(name); err != nil {
						t.Fatal(err)
					}
				}
			})

			var err error
			var stdout string
			stderr := captureStderr(func() {
				stdout = captureOutput(func() {
					err = NewRoot("v1.2.3").Execute([]string{ProjectCmd.Name, "list", "--page-size", "1"})
				})
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stdout != testCase.wantStdout {
				t.Errorf("stdout:\n%q\nwant:\n%q", stdout, testCase.wantStdout)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr:\n%q\nwant:\n%q", stderr, testCase.wantStderr)
			}
		})
	}
}
