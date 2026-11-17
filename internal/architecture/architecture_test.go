package architecture

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	uiDir       = "internal/ui/"
	tuiDir      = "internal/ui/tui/"
	osPackage   = "os"
	commonPath  = modulePath + "internal/common"
	configPath  = modulePath + "internal/config"
	adaptersDir = modulePath + "internal/adapters/"
)

// rules are the layer rules of drudge. They have no exceptions.
var rules = []rule{
	{
		name:             "interfaces-no-adapters",
		dirs:             []string{uiDir},
		forbiddenImports: []string{adaptersDir, "os/exec", "net/http"},
		reason:           "an interface gets adapters from internal/app, an adapter builds a child process",
	},
	{
		name:             "interfaces-terminal-only",
		dirs:             []string{uiDir},
		allowedSelectors: map[string][]string{osPackage: {"Stdin", "Stdout", "Stderr", "Exit", "Signal"}},
		reason:           "an interface touches only the terminal, a service reaches files, environment variables and processes through a port",
	},
	{
		name: "interfaces-no-file-io",
		dirs: []string{uiDir},
		forbiddenSelectors: map[string][]string{
			commonPath: {"ReadFile", "WriteFile", "ReadJSON", "WriteJSON", "Exists", "RemoveAll", "EnsureDir", "CopyPath", "WriteFileWithFrontMatter"},
		},
		reason: "an adapter does the file I/O and internal/app wires it",
	},
	{
		name:               "interfaces-no-config-loading",
		dirs:               []string{uiDir},
		forbiddenSelectors: map[string][]string{configPath: {"Load", "Resolve"}},
		reason:             "internal/app loads the config and resolves its values",
	},
	{
		name: "domain-inward-only",
		dirs: []string{
			"internal/task/", "internal/project/", "internal/drudger/", "internal/git/",
			"internal/remote/", "internal/release/", "internal/setup/", "internal/skill/",
		},
		forbiddenImports: []string{adaptersDir, modulePath + uiDir, modulePath + "internal/app"},
		reason:           "the domain defines ports, adapters implement them and internal/app wires them",
	},
	{
		name:             "charm-only-in-tui",
		dirs:             []string{""},
		exemptDirs:       []string{tuiDir},
		forbiddenImports: []string{"charm.land/", "github.com/charmbracelet/"},
		reason:           "only the TUI uses the charm libraries",
	},
}

func TestArchitecture(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	violations, err := checkModule(root, rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, found := range violations {
		t.Error(found)
	}
}

func TestCheckModule(t *testing.T) {
	cases := []struct {
		name     string
		file     string
		relPath  string
		wantUsed []string
	}{
		{
			name:     "forbidden import",
			file:     "adapter_import.go",
			relPath:  "internal/ui/cli/adapter_import.go",
			wantUsed: []string{adaptersDir + "remote"},
		},
		{
			name:     "aliased import",
			file:     "aliased_import.go",
			relPath:  "internal/ui/cli/aliased_import.go",
			wantUsed: []string{configPath + ".LoadLocal"},
		},
		{
			name:    "allowed os.Stdout",
			file:    "os_stdout.go",
			relPath: "internal/ui/cli/os_stdout.go",
		},
		{
			name:     "forbidden os.ReadFile",
			file:     "os_readfile.go",
			relPath:  "internal/ui/cli/os_readfile.go",
			wantUsed: []string{"os.ReadFile"},
		},
		{
			name:     "forbidden config.ResolveTaskPageSize",
			file:     "config_resolve.go",
			relPath:  "internal/ui/cli/config_resolve.go",
			wantUsed: []string{configPath + ".ResolveTaskPageSize"},
		},
		{
			name:    "allowed config.TaskPageSizeKey",
			file:    "config_key.go",
			relPath: "internal/ui/cli/config_key.go",
		},
		{
			name:    "skipped test file",
			file:    "os_readfile.go",
			relPath: "internal/ui/cli/os_readfile_test.go",
		},
		{
			name:     "charm outside the TUI",
			file:     "charm_import.go",
			relPath:  "internal/task/charm_import.go",
			wantUsed: []string{"charm.land/lipgloss/v2"},
		},
		{
			name:    "charm in the TUI",
			file:    "charm_import.go",
			relPath: "internal/ui/tui/charm_import.go",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			copySnippet(t, testCase.file, filepath.Join(root, filepath.FromSlash(testCase.relPath)))
			violations, err := checkModule(root, rules)
			if err != nil {
				t.Fatal(err)
			}
			var gotUsed []string
			for _, found := range violations {
				gotUsed = append(gotUsed, found.used)
			}
			if !slices.Equal(gotUsed, testCase.wantUsed) {
				t.Errorf("used = %v, want %v", gotUsed, testCase.wantUsed)
			}
		})
	}
}

// copySnippet copies the snippet file of the test data to target.
func copySnippet(t *testing.T, file string, target string) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(testdataDir, file))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, source, 0o644); err != nil {
		t.Fatal(err)
	}
}

// moduleRoot returns the closest directory above the working directory that
// holds a go.mod file.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, goModFile)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s in any directory above the working directory", goModFile)
		}
		dir = parent
	}
}
