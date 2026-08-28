package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"drudge/internal/common"
)

func TestProjectInit_ProjectCreatedGlobally(t *testing.T) {
	home, cleanup := setupHome(t)
	defer cleanup()

	err := projectInit([]string{"Test Project"})
	if err != nil {
		t.Fatalf("projectInit: %v", err)
	}

	globalProjectDir := filepath.Join(common.ProjectsDir(home), "test-project")
	projFile := filepath.Join(globalProjectDir, "project.json")

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

	configPath := filepath.Join(common.DrudgeDir(home), common.DrudgeConfigName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("could not read local config: %v", err)
	}

	var cfg projectConfig
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

	configPath := filepath.Join(common.DrudgeDir(home), common.DrudgeConfigName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("could not read config: %v", err)
	}

	var cfg projectConfig
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

	return dir, func() {}
}
