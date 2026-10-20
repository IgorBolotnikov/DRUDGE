package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeSetupRecorder records the setups it is handed.
type fakeSetupRecorder struct {
	wasCalled bool
	setups    map[string][]string
}

func (recorder *fakeSetupRecorder) RecordSetup(setups map[string][]string) error {
	recorder.wasCalled = true
	recorder.setups = setups
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

			got, err := service.DetectSetup(projectDir, []Repository{{Path: "."}}, DetectSetupOptions{})
			if err != nil {
				t.Fatalf("DetectSetup: %v", err)
			}

			want := []SetupDetection{{Repository: ".", Lines: test.want}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("DetectSetup = %+v, want %+v", got, want)
			}
			wantSetups := map[string][]string{".": setupCommands(test.want)}
			if !reflect.DeepEqual(recorder.setups, wantSetups) {
				t.Errorf("recorded %+v, want %+v", recorder.setups, wantSetups)
			}
		})
	}
}

func TestDetectSetup_Options(t *testing.T) {
	// Every repository holds an npm lockfile.
	detected := linesFrom("package-lock.json", "npm ci")

	tests := []struct {
		name         string
		repositories []Repository
		options      DetectSetupOptions
		want         []SetupDetection
		// wantSetups is what the recorder is handed, and nil when it is not
		// called.
		wantSetups map[string][]string
	}{
		{
			name:         "a missing setup is filled",
			repositories: []Repository{{Path: "api"}},
			want:         []SetupDetection{{Repository: "api", Lines: detected}},
			wantSetups:   map[string][]string{"api": {"npm ci"}},
		},
		{
			name:         "an empty setup and a hand-written one are kept",
			repositories: []Repository{{Path: "api", Setup: []string{}}, {Path: "ui", Setup: []string{"make deps"}}, {Path: "web"}},
			want: []SetupDetection{
				{Repository: "api", IsKept: true},
				{Repository: "ui", IsKept: true},
				{Repository: "web", Lines: detected},
			},
			wantSetups: map[string][]string{"web": {"npm ci"}},
		},
		{
			name:         "every setup kept writes nothing",
			repositories: []Repository{{Path: "api", Setup: []string{}}},
			want:         []SetupDetection{{Repository: "api", IsKept: true}},
		},
		{
			name:         "force replaces every setup",
			repositories: []Repository{{Path: "api", Setup: []string{}}, {Path: "ui", Setup: []string{"make deps"}}},
			options:      DetectSetupOptions{IsForced: true},
			want:         []SetupDetection{{Repository: "api", Lines: detected}, {Repository: "ui", Lines: detected}},
			wantSetups:   map[string][]string{"api": {"npm ci"}, "ui": {"npm ci"}},
		},
		{
			name:         "a dry run writes nothing",
			repositories: []Repository{{Path: "api"}, {Path: "ui", Setup: []string{"make deps"}}},
			options:      DetectSetupOptions{IsDryRun: true},
			want:         []SetupDetection{{Repository: "api", Lines: detected}, {Repository: "ui", IsKept: true}},
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

			got, err := service.DetectSetup(projectDir, test.repositories, test.options)
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

	_, err := service.DetectSetup(t.TempDir(), nil, DetectSetupOptions{})

	if err == nil || !strings.Contains(err.Error(), RepositoriesKey) {
		t.Errorf("error = %v, want it to name %q", err, RepositoriesKey)
	}
}
