package project

import (
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/exec"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/gitcli"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
)

// fakeSetupRecorder records the setups it is handed.
type fakeSetupRecorder struct {
	wasCalled bool
	setups    map[string]DetectedSetup
	envNames  []string
}

func (recorder *fakeSetupRecorder) RecordSetup(setups map[string]DetectedSetup, envNames []string) error {
	recorder.wasCalled = true
	recorder.setups = setups
	recorder.envNames = envNames
	return nil
}

// writeFiles creates files under dir, each path mapped to its content.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func miseLines(trigger string) []SetupLine {
	return linesFrom(trigger, installMiseLine, enableIdiomaticPinsLine, miseInstallLine, addMiseShimsLine)
}

func TestDetectSetup_Rules(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []SetupLine
	}{
		{
			name:  "a Go module pins nothing",
			files: map[string]string{"go.mod": "module example.com/demo\n\ngo 1.27.0\n"},
			want:  []SetupLine{},
		},
		{
			name:  "an empty repository",
			files: map[string]string{},
			want:  []SetupLine{},
		},
		{
			name:  "the engines of a package.json pin nothing",
			files: map[string]string{"package.json": `{"engines": {"node": ">=22"}}`},
			want:  []SetupLine{},
		},
		{
			name:  "a tool versions file",
			files: map[string]string{".tool-versions": "node 22.11.0\n"},
			want:  miseLines(".tool-versions"),
		},
		{
			name:  "a mise config file",
			files: map[string]string{"mise.toml": "[tools]\nnode = \"22\"\n"},
			want:  miseLines("mise.toml"),
		},
		{
			name:  "a hidden mise config file",
			files: map[string]string{".mise.toml": "[tools]\nnode = \"22\"\n"},
			want:  miseLines(".mise.toml"),
		},
		{
			name:  "an nvmrc",
			files: map[string]string{".nvmrc": "22\n"},
			want:  miseLines(".nvmrc"),
		},
		{
			name:  "a node version file",
			files: map[string]string{".node-version": "22\n"},
			want:  miseLines(".node-version"),
		},
		{
			name:  "a python version file",
			files: map[string]string{".python-version": "3.12\n"},
			want:  miseLines(".python-version"),
		},
		{
			name:  "the first pin file names the trigger",
			files: map[string]string{".nvmrc": "22\n", ".tool-versions": "python 3.12\n"},
			want:  miseLines(".tool-versions"),
		},
		{
			name:  "an npm lockfile",
			files: map[string]string{"package-lock.json": "{}"},
			want:  linesFrom("package-lock.json", "npm ci"),
		},
		{
			name:  "an npm lockfile with a version file",
			files: map[string]string{"package-lock.json": "{}", ".nvmrc": "22\n"},
			want:  append(miseLines(".nvmrc"), linesFrom("package-lock.json", "npm ci")...),
		},
		{
			name:  "unpinned yarn comes from corepack",
			files: map[string]string{"yarn.lock": ""},
			want:  linesFrom("yarn.lock", `corepack enable --install-directory "$HOME/.local/bin"`, "yarn install --frozen-lockfile"),
		},
		{
			name:  "yarn pinned in the tool versions file comes from mise",
			files: map[string]string{"yarn.lock": "", ".tool-versions": "nodejs 22.11.0\nyarn 1.22.22 # classic\n"},
			want:  append(miseLines(".tool-versions"), linesFrom("yarn.lock", "yarn install --frozen-lockfile")...),
		},
		{
			name:  "yarn pinned in a mise config file comes from mise",
			files: map[string]string{"yarn.lock": "", "mise.toml": "[env]\nyarn = \"no\"\n\n[tools]\nnode = \"22\"\n\"yarn\" = \"1.22.22\"\n"},
			want:  append(miseLines("mise.toml"), linesFrom("yarn.lock", "yarn install --frozen-lockfile")...),
		},
		{
			name:  "yarn named outside the tools table is not pinned",
			files: map[string]string{"yarn.lock": "", "mise.toml": "[env]\nyarn = \"1.22.22\"\n"},
			want: append(
				miseLines("mise.toml"),
				linesFrom("yarn.lock", `corepack enable --install-directory "$HOME/.local/bin"`, "yarn install --frozen-lockfile")...,
			),
		},
		{
			name:  "Berry yarn comes from corepack",
			files: map[string]string{"yarn.lock": "", ".yarnrc.yml": "nodeLinker: node-modules\n", ".tool-versions": "yarn 4.5.0\n"},
			want: append(miseLines(".tool-versions"), []SetupLine{
				{Command: `corepack enable --install-directory "$HOME/.local/bin"`, Trigger: ".yarnrc.yml"},
				{Command: "yarn install --immutable", Trigger: "yarn.lock"},
			}...),
		},
		{
			name:  "a pnpm lockfile",
			files: map[string]string{"pnpm-lock.yaml": ""},
			want:  linesFrom("pnpm-lock.yaml", `corepack enable --install-directory "$HOME/.local/bin"`, "pnpm install --frozen-lockfile"),
		},
		{
			name:  "a uv lockfile",
			files: map[string]string{"uv.lock": "", "pyproject.toml": "[project]\nname = \"demo\"\n"},
			want:  linesFrom("uv.lock", "uv sync --frozen"),
		},
		{
			name:  "unpinned poetry comes from uv",
			files: map[string]string{"poetry.lock": ""},
			want:  linesFrom("poetry.lock", "uv tool install poetry", "poetry install"),
		},
		{
			name:  "poetry pinned in the tool versions file comes from mise",
			files: map[string]string{"poetry.lock": "", ".tool-versions": "python 3.12\npoetry 1.8.4\n"},
			want:  append(miseLines(".tool-versions"), linesFrom("poetry.lock", "poetry install")...),
		},
		{
			name:  "a pyproject with a poetry table",
			files: map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\n\n[tool.poetry]\npackage-mode = false\n"},
			want:  linesFrom("pyproject.toml", "uv tool install poetry", "poetry install"),
		},
		{
			name:  "a pyproject with only a poetry subtable",
			files: map[string]string{"pyproject.toml": "[tool.poetry.dependencies]\npython = \"^3.12\"\n"},
			want:  linesFrom("pyproject.toml", "uv tool install poetry", "poetry install"),
		},
		{
			name:  "a pyproject with no poetry table",
			files: map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\n\n[tool.ruff]\nline-length = 100\n"},
			want:  []SetupLine{},
		},
		{
			name:  "a requirements file",
			files: map[string]string{"requirements.txt": "requests\n"},
			want:  linesFrom("requirements.txt", "python3 -m venv .venv", ".venv/bin/pip install -r requirements.txt"),
		},
		{
			name:  "the first lockfile wins",
			files: map[string]string{"yarn.lock": "", "package-lock.json": "{}", "requirements.txt": ""},
			want:  linesFrom("package-lock.json", "npm ci"),
		},
		{
			name:  "files below the root are not read",
			files: map[string]string{"web/package-lock.json": "{}", "web/.nvmrc": "22\n"},
			want:  []SetupLine{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			writeFiles(t, projectDir, test.files)
			recorder := &fakeSetupRecorder{}
			service := NewProjectService(nil, nil, recorder, nil, &fakeProgress{})

			got, err := service.DetectSetup(projectDir, []Repository{{Path: "."}}, nil, DetectSetupOptions{})
			if err != nil {
				t.Fatalf("DetectSetup: %v", err)
			}

			want := []SetupDetection{{Repository: ".", Lines: test.want, CopyPaths: []string{}}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("DetectSetup = %+v, want %+v", got, want)
			}
			wantSetups := map[string]DetectedSetup{".": {Setup: setupCommands(test.want), Copy: []string{}}}
			if !reflect.DeepEqual(recorder.setups, wantSetups) {
				t.Errorf("recorded %+v, want %+v", recorder.setups, wantSetups)
			}
		})
	}
}

func TestDetectSetup_Options(t *testing.T) {
	// Every repository holds an npm lockfile and nothing to copy.
	detected := linesFrom("package-lock.json", "npm ci")

	tests := []struct {
		name         string
		repositories []Repository
		options      DetectSetupOptions
		want         []SetupDetection
		// wantSetups is what the recorder is handed, and nil when it is not
		// called.
		wantSetups map[string]DetectedSetup
	}{
		{
			name:         "a missing setup is filled",
			repositories: []Repository{{Path: "api"}},
			want:         []SetupDetection{{Repository: "api", Lines: detected, CopyPaths: []string{}}},
			wantSetups:   map[string]DetectedSetup{"api": {Setup: []string{"npm ci"}, Copy: []string{}}},
		},
		{
			name:         "an empty setup and a hand-written one are kept",
			repositories: []Repository{{Path: "api", Setup: []string{}}, {Path: "ui", Setup: []string{"make deps"}}, {Path: "web"}},
			want: []SetupDetection{
				{Repository: "api", IsKept: true},
				{Repository: "ui", IsKept: true},
				{Repository: "web", Lines: detected, CopyPaths: []string{}},
			},
			wantSetups: map[string]DetectedSetup{"web": {Setup: []string{"npm ci"}, Copy: []string{}}},
		},
		{
			name:         "a hand-written copy is kept",
			repositories: []Repository{{Path: "api", Copy: []string{"secrets.json"}}},
			want:         []SetupDetection{{Repository: "api", Lines: detected, IsCopyKept: true}},
			wantSetups:   map[string]DetectedSetup{"api": {Setup: []string{"npm ci"}}},
		},
		{
			name:         "every setup kept writes nothing",
			repositories: []Repository{{Path: "api", Setup: []string{}}},
			want:         []SetupDetection{{Repository: "api", IsKept: true}},
		},
		{
			name: "force replaces every setup and copy",
			repositories: []Repository{
				{Path: "api", Setup: []string{}},
				{Path: "ui", Setup: []string{"make deps"}, Copy: []string{"secrets.json"}},
			},
			options: DetectSetupOptions{IsForced: true},
			want: []SetupDetection{
				{Repository: "api", Lines: detected, CopyPaths: []string{}},
				{Repository: "ui", Lines: detected, CopyPaths: []string{}},
			},
			wantSetups: map[string]DetectedSetup{
				"api": {Setup: []string{"npm ci"}, Copy: []string{}},
				"ui":  {Setup: []string{"npm ci"}, Copy: []string{}},
			},
		},
		{
			name:         "a dry run writes nothing",
			repositories: []Repository{{Path: "api"}, {Path: "ui", Setup: []string{"make deps"}}},
			options:      DetectSetupOptions{IsDryRun: true},
			want:         []SetupDetection{{Repository: "api", Lines: detected, CopyPaths: []string{}}, {Repository: "ui", IsKept: true}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			for _, repository := range test.repositories {
				writeFiles(t, projectDir, map[string]string{filepath.Join(repository.Path, "package-lock.json"): "{}"})
			}
			recorder := &fakeSetupRecorder{}
			service := NewProjectService(nil, nil, recorder, nil, &fakeProgress{})

			got, err := service.DetectSetup(projectDir, test.repositories, nil, test.options)
			if err != nil {
				t.Fatalf("DetectSetup: %v", err)
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("DetectSetup = %+v, want %+v", got, test.want)
			}
			if recorder.wasCalled != (test.wantSetups != nil) {
				t.Errorf("recorder called: %v, want %v", recorder.wasCalled, test.wantSetups != nil)
			}
			if test.wantSetups != nil && !reflect.DeepEqual(recorder.setups, test.wantSetups) {
				t.Errorf("recorded %+v, want %+v", recorder.setups, test.wantSetups)
			}
		})
	}
}

func TestDetectSetup_RefusesAProjectWithNoRepositories(t *testing.T) {
	service := NewProjectService(nil, nil, &fakeSetupRecorder{}, nil, &fakeProgress{})

	_, err := service.DetectSetup(t.TempDir(), nil, nil, DetectSetupOptions{})

	if err == nil || !strings.Contains(err.Error(), RepositoriesKey) {
		t.Errorf("error = %v, want it to name %q", err, RepositoriesKey)
	}
}

func TestDetectSetup_CopyPaths(t *testing.T) {
	tests := []struct {
		name string
		// gitignore is the ignore file of the repository.
		gitignore string
		files     map[string]string
		// tracked are files added to the index, whatever the ignore file says.
		tracked []string
		want    []string
	}{
		{
			name:      "an ignored env file",
			gitignore: ".env\n",
			files:     map[string]string{".env": "TOKEN=1\n"},
			want:      []string{".env"},
		},
		{
			name:      "every ignored file starting with .env",
			gitignore: ".env*\n",
			files:     map[string]string{".env": "", ".env.local": "", ".environment": ""},
			want:      []string{".env", ".env.local", ".environment"},
		},
		{
			name:      "a tracked env example is left out",
			gitignore: ".env*\n",
			files:     map[string]string{".env": "", ".env.example": "TOKEN=\n"},
			tracked:   []string{".env.example"},
			want:      []string{".env"},
		},
		{
			name:      "an env file git does not ignore is left out",
			gitignore: "",
			files:     map[string]string{".env.example": "TOKEN=\n"},
			want:      []string{},
		},
		{
			name:      "an envrc is left out",
			gitignore: ".env*\n",
			files:     map[string]string{".envrc": "use mise\n"},
			want:      []string{},
		},
		{
			name:      "an envs directory",
			gitignore: ".envs/\n",
			files:     map[string]string{".envs/.local/.django": "DEBUG=1\n"},
			want:      []string{".envs"},
		},
		{
			name:      "an ignored env directory other than envs is left out",
			gitignore: ".env/\n",
			files:     map[string]string{".env/bin/python": ""},
			want:      []string{},
		},
		{
			name:      "files below the root are not read",
			gitignore: ".env\n",
			files:     map[string]string{"web/.env": ""},
			want:      []string{},
		},
		{
			name:      "nothing to copy",
			gitignore: "node_modules/\n",
			files:     map[string]string{"package-lock.json": "{}"},
			want:      []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			runGit(t, projectDir, "init", "-q")
			writeFiles(t, projectDir, map[string]string{".gitignore": test.gitignore})
			writeFiles(t, projectDir, test.files)
			for _, path := range test.tracked {
				runGit(t, projectDir, "add", "--force", path)
			}
			recorder := &fakeSetupRecorder{}
			gitOps := gitcli.New(exec.NewCommandRunner(), git.Timeouts{Command: 30 * time.Second})
			service := NewProjectService(nil, nil, recorder, gitOps, &fakeProgress{})

			got, err := service.DetectSetup(projectDir, []Repository{{Path: "."}}, nil, DetectSetupOptions{})
			if err != nil {
				t.Fatalf("DetectSetup: %v", err)
			}

			if !reflect.DeepEqual(got[0].CopyPaths, test.want) {
				t.Errorf("copy paths = %q, want %q", got[0].CopyPaths, test.want)
			}
			if !reflect.DeepEqual(recorder.setups["."].Copy, test.want) {
				t.Errorf("recorded copy paths %q, want %q", recorder.setups["."].Copy, test.want)
			}
		})
	}
}

func TestDetectSetup_EnvNames(t *testing.T) {
	tests := []struct {
		name         string
		repositories []Repository
		files        map[string]string
		// listed is the Drudger env before detection.
		listed []string
		want   []SetupDetection
		// wantEnvNames is what the recorder is handed.
		wantEnvNames []string
	}{
		{
			name:         "an npmrc with one name",
			repositories: []Repository{{Path: "api", Setup: []string{}}},
			files:        map[string]string{"api/.npmrc": "//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}\n"},
			want:         []SetupDetection{{Repository: "api", IsKept: true, EnvNames: []EnvName{{Name: "NODE_AUTH_TOKEN", Trigger: ".npmrc"}}}},
			wantEnvNames: []string{"NODE_AUTH_TOKEN"},
		},
		{
			name:         "an npmrc with two names",
			repositories: []Repository{{Path: "api", Setup: []string{}}},
			files: map[string]string{"api/.npmrc": "@acme:registry=https://${ACME_REGISTRY}/\n" +
				"# //old.example.com/:_authToken=${OLD_TOKEN}\n" +
				"; ${OLD_TOKEN}\n" +
				"//${ACME_REGISTRY}/:_authToken=${ACME_TOKEN}\n"},
			want: []SetupDetection{{Repository: "api", IsKept: true, EnvNames: []EnvName{
				{Name: "ACME_REGISTRY", Trigger: ".npmrc"},
				{Name: "ACME_TOKEN", Trigger: ".npmrc"},
			}}},
			wantEnvNames: []string{"ACME_REGISTRY", "ACME_TOKEN"},
		},
		{
			name:         "an npmrc with no names",
			repositories: []Repository{{Path: "api", Setup: []string{}}},
			files:        map[string]string{"api/.npmrc": "save-exact=true\n"},
			want:         []SetupDetection{{Repository: "api", IsKept: true}},
		},
		{
			name:         "no npmrc",
			repositories: []Repository{{Path: "api", Setup: []string{}}},
			want:         []SetupDetection{{Repository: "api", IsKept: true}},
		},
		{
			name:         "an npmrc below the root is not read",
			repositories: []Repository{{Path: "api", Setup: []string{}}},
			files:        map[string]string{"api/web/.npmrc": "//registry.npmjs.org/:_authToken=${NPM_TOKEN}\n"},
			want:         []SetupDetection{{Repository: "api", IsKept: true}},
		},
		{
			name:         "a listed name is not added again",
			repositories: []Repository{{Path: "api", Setup: []string{}}},
			files:        map[string]string{"api/.npmrc": "//registry.npmjs.org/:_authToken=${NPM_TOKEN}\n"},
			listed:       []string{"NPM_TOKEN"},
			want:         []SetupDetection{{Repository: "api", IsKept: true}},
		},
		{
			name:         "a name two repositories need is added once",
			repositories: []Repository{{Path: "api", Setup: []string{}}, {Path: "ui", Setup: []string{}}},
			files: map[string]string{
				"api/.npmrc": "//registry.npmjs.org/:_authToken=${NPM_TOKEN}\n",
				"ui/.npmrc":  "//registry.npmjs.org/:_authToken=${NPM_TOKEN}\n",
			},
			want: []SetupDetection{
				{Repository: "api", IsKept: true, EnvNames: []EnvName{{Name: "NPM_TOKEN", Trigger: ".npmrc"}}},
				{Repository: "ui", IsKept: true},
			},
			wantEnvNames: []string{"NPM_TOKEN"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			for _, repository := range test.repositories {
				if err := os.MkdirAll(filepath.Join(projectDir, repository.Path), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			writeFiles(t, projectDir, test.files)
			recorder := &fakeSetupRecorder{}
			service := NewProjectService(nil, nil, recorder, nil, &fakeProgress{})

			got, err := service.DetectSetup(projectDir, test.repositories, test.listed, DetectSetupOptions{})
			if err != nil {
				t.Fatalf("DetectSetup: %v", err)
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("DetectSetup = %+v, want %+v", got, test.want)
			}
			if recorder.wasCalled != (test.wantEnvNames != nil) {
				t.Errorf("recorder called: %v, want %v", recorder.wasCalled, test.wantEnvNames != nil)
			}
			if !reflect.DeepEqual(recorder.envNames, test.wantEnvNames) {
				t.Errorf("recorded env names %q, want %q", recorder.envNames, test.wantEnvNames)
			}
		})
	}
}

// runGit runs git in dir with the git variables of the environment cleared,
// so a git hook running the tests does not point it at another repository.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := osexec.Command("git", args...)
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
