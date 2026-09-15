package cmd

import (
	"encoding/json"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"drudge/internal/adapters/persistence"
	"drudge/internal/common"
	"drudge/internal/config"
)

const testDefaultBranch = "main"

func TestProjectInit_ProjectCreatedGlobally(t *testing.T) {
	home, cleanup := setupHome(t)
	defer cleanup()

	err := projectInit([]string{"Test Project"})
	if err != nil {
		t.Fatalf("projectInit: %v", err)
	}

	globalProjectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	projFile := filepath.Join(globalProjectDir, persistence.ProjectConfigFile)

	exists, err := os.Stat(projFile)
	if err != nil {
		t.Fatalf("global project.json not found: %v", err)
	}
	if exists == nil {
		t.Fatal("project.json should exist")
	}
}

func TestProjectInit_LocalConfigCreated(t *testing.T) {
	home, cleanup := setupHome(t)
	defer cleanup()

	err := projectInit([]string{"Test Project"})
	if err != nil {
		t.Fatalf("projectInit: %v", err)
	}

	configPath := filepath.Join(common.DrudgeDir(home), common.GloablConfigName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("could not read local config: %v", err)
	}

	var cfg config.LocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("could not parse config: %v", err)
	}

	if cfg.ProjectSlug != "test-project" {
		t.Errorf("expected slug 'test-project', got %q", cfg.ProjectSlug)
	}
}

func TestProjectInit_LocalDrudgeDirCreated(t *testing.T) {
	home, cleanup := setupHome(t)
	defer cleanup()

	err := projectInit([]string{"Test Project"})
	if err != nil {
		t.Fatalf("projectInit: %v", err)
	}

	dirPath := common.DrudgeDir(home)

	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf(".drudge dir not found: %v", err)
	}
	if !info.IsDir() {
		t.Error(".drudge should be a directory")
	}
}

func TestProjectInit_NoArgs(t *testing.T) {
	_, cleanup := setupHome(t)
	defer cleanup()

	err := projectInit([]string{})
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestProjectInit_SlugFromName(t *testing.T) {
	home, cleanup := setupHome(t)
	defer cleanup()

	err := projectInit([]string{"My Cool App"})
	if err != nil {
		t.Fatalf("projectInit: %v", err)
	}

	configPath := filepath.Join(common.DrudgeDir(home), common.GloablConfigName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("could not read config: %v", err)
	}

	var cfg config.LocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("could not parse config: %v", err)
	}

	if cfg.ProjectSlug != "my-cool-app" {
		t.Errorf("expected slug 'my-cool-app', got %q", cfg.ProjectSlug)
	}
}

func setupHome(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// Change to the temp dir so relative .drudge paths resolve inside it
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origCwd) })

	initGitRepo(t, dir, testDefaultBranch)

	return dir, func() {}
}

// initGitRepo makes dir a repository that answers for its default branch, the
// way a clone does. `drg project init` refuses a directory holding no
// repository.
func initGitRepo(t *testing.T, dir string, branch string) {
	t.Helper()
	runGit(t, dir, "init", "-q", "-b", branch)
	runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/"+branch)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := osexec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, output)
	}
}

func TestProjectInit_RecordsTheProjectDirectoryAsOneRepository(t *testing.T) {
	home, cleanup := setupHome(t)
	defer cleanup()

	output := captureOutput(func() {
		if err := projectInit([]string{"Test Project"}); err != nil {
			t.Fatalf("projectInit: %v", err)
		}
	})

	cfg := readLocalConfig(t, home)
	want := config.Repository{Path: "."}
	if len(cfg.Repositories) != 1 || cfg.Repositories[0] != want {
		t.Fatalf("Repositories = %+v, want one %+v", cfg.Repositories, want)
	}
	if !strings.Contains(output, testDefaultBranch) {
		t.Errorf("output = %q, want it to name the default branch %q", output, testDefaultBranch)
	}
}

func TestProjectInit_RecordsOneEntryPerRepositoryDirectory(t *testing.T) {
	home, cleanup := setupHome(t)
	defer cleanup()

	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	removeGitDir(t, projectDir)
	for _, name := range []string{"api", "ui"} {
		subdir := filepath.Join(projectDir, name)
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatalf("could not create %s: %v", name, err)
		}
		initGitRepo(t, subdir, testDefaultBranch)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "docs"), 0o755); err != nil {
		t.Fatalf("could not create docs: %v", err)
	}

	captureOutput(func() {
		if err := projectInit([]string{"Test Project"}); err != nil {
			t.Fatalf("projectInit: %v", err)
		}
	})

	cfg := readLocalConfig(t, home)
	want := []config.Repository{{Path: "api"}, {Path: "ui"}}
	if len(cfg.Repositories) != len(want) {
		t.Fatalf("Repositories = %+v, want %+v", cfg.Repositories, want)
	}
	for index, repository := range cfg.Repositories {
		if repository != want[index] {
			t.Errorf("repository %d = %+v, want %+v", index, repository, want[index])
		}
	}
}

func TestProjectInit_NoRepositoryAnywhere_NamesTheDirectory(t *testing.T) {
	_, cleanup := setupHome(t)
	defer cleanup()

	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	removeGitDir(t, projectDir)

	err = projectInit([]string{"Test Project"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), projectDir) {
		t.Errorf("error = %q, want it to name %q", err, projectDir)
	}
}

// removeGitDir turns a repository back into a plain directory.
func removeGitDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("could not remove the git directory of %s: %v", dir, err)
	}
}

func readLocalConfig(t *testing.T, home string) config.LocalConfig {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(common.DrudgeDir(home), common.GloablConfigName))
	if err != nil {
		t.Fatalf("could not read local config: %v", err)
	}

	var cfg config.LocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("could not parse local config: %v", err)
	}
	return cfg
}
