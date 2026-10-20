package drudger

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// settingsWithCopies are the settings of a project whose api and ui
// repositories list paths to copy and whose api repository has a setup
// command.
func settingsWithCopies() Settings {
	settings := testSettings()
	settings.Repositories = []project.Repository{
		{Path: "api", Copy: []string{".env", "certs"}, Setup: []string{"npm ci"}},
		{Path: "ui", Copy: []string{".env.local"}},
		{Path: "docs"},
	}
	return settings
}

// registerWrittenWorktrees registers the repository directories a test wrote
// into the workspace of slot 1 as worktrees, so a launch reuses them.
func registerWrittenWorktrees(service *testService, projectDir string) {
	for _, repository := range service.settings.Repositories {
		worktree := filepath.Join(slotRoot(projectDir, 1), repository.Path)
		if _, err := os.Stat(worktree); err == nil {
			service.git.registerWorktree(worktree)
		}
	}
}

// writeFiles writes files under root, keyed by their path relative to root.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("could not create the parent of %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("could not write %s: %v", full, err)
		}
	}
}

func TestDrudgerService_RunTask_CopiesLocalFilesIntoTheWorktrees(t *testing.T) {
	cases := []struct {
		name         string
		repositories []string
		// hostFiles are the files of the project directory.
		hostFiles map[string]string
		// worktreeFiles are the files of the workspace before the launch.
		worktreeFiles map[string]string
		// wantFiles are the files of the workspace after the launch.
		wantFiles map[string]string
		// absentFiles are paths of the workspace the launch leaves missing.
		absentFiles []string
		wantCopied  []LocalCopy
		wantMissing []LocalCopy
	}{
		{
			name:         "files and directories land in the worktree",
			repositories: []string{"api"},
			hostFiles:    map[string]string{"api/.env": "TOKEN=1", "api/certs/dev.pem": "dev", "api/certs/ca/root.pem": "root"},
			wantFiles:    map[string]string{"api/.env": "TOKEN=1", "api/certs/dev.pem": "dev", "api/certs/ca/root.pem": "root"},
			wantCopied:   []LocalCopy{{Repository: "api", Path: ".env"}, {Repository: "api", Path: "certs"}},
		},
		{
			name:          "a copy overwrites what the worktree holds",
			repositories:  []string{"api"},
			hostFiles:     map[string]string{"api/.env": "TOKEN=new", "api/certs/dev.pem": "new"},
			worktreeFiles: map[string]string{"api/.env": "TOKEN=old", "api/certs/dev.pem": "old"},
			wantFiles:     map[string]string{"api/.env": "TOKEN=new", "api/certs/dev.pem": "new"},
			wantCopied:    []LocalCopy{{Repository: "api", Path: ".env"}, {Repository: "api", Path: "certs"}},
		},
		{
			name:         "a path missing from the repository is reported and the rest is copied",
			repositories: []string{"api"},
			hostFiles:    map[string]string{"api/certs/dev.pem": "dev"},
			wantFiles:    map[string]string{"api/certs/dev.pem": "dev"},
			absentFiles:  []string{"api/.env"},
			wantCopied:   []LocalCopy{{Repository: "api", Path: "certs"}},
			wantMissing:  []LocalCopy{{Repository: "api", Path: ".env"}},
		},
		{
			name:         "only the repositories the task changes get copies",
			repositories: []string{"ui", "docs"},
			hostFiles:    map[string]string{"api/.env": "TOKEN=1", "ui/.env.local": "PORT=3000"},
			wantFiles:    map[string]string{"ui/.env.local": "PORT=3000"},
			absentFiles:  []string{"api/.env"},
			wantCopied:   []LocalCopy{{Repository: "ui", Path: ".env.local"}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			writeFiles(t, projectDir, testCase.hostFiles)
			writeFiles(t, slotRoot(projectDir, 1), testCase.worktreeFiles)
			taskToRun := todoTask()
			taskToRun.Repositories = testCase.repositories
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(settingsWithCopies(), commands, taskToRun)
			registerWrittenWorktrees(service, projectDir)

			if err := service.RunTask(testProjectSlug, taskToRun.ID, false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for path, want := range testCase.wantFiles {
				got, err := os.ReadFile(filepath.Join(slotRoot(projectDir, 1), path))
				if err != nil {
					t.Fatalf("expected %s in the workspace: %v", path, err)
				}
				if string(got) != want {
					t.Errorf("expected %s to hold %q, got %q", path, want, got)
				}
			}
			for _, path := range testCase.absentFiles {
				if _, err := os.Stat(filepath.Join(slotRoot(projectDir, 1), path)); !os.IsNotExist(err) {
					t.Errorf("expected no %s in the workspace, got %v", path, err)
				}
			}

			var copied []LocalCopy
			for _, event := range reportedEvents[LocalFileCopied](service.progress) {
				copied = append(copied, event.LocalCopy)
			}
			if !slices.Equal(copied, testCase.wantCopied) {
				t.Errorf("expected the copies %+v reported, got %+v", testCase.wantCopied, copied)
			}
			var missing []LocalCopy
			for _, event := range reportedEvents[LocalFileMissing](service.progress) {
				missing = append(missing, event.LocalCopy)
				if want := filepath.Join(projectDir, event.Repository); event.Dir != want {
					t.Errorf("expected the missing path reported in %s, got %s", want, event.Dir)
				}
			}
			if !slices.Equal(missing, testCase.wantMissing) {
				t.Errorf("expected the missing paths %+v reported, got %+v", testCase.wantMissing, missing)
			}
			if taskToRun.Status != task.StatusInProgress {
				t.Errorf("expected status %q, got %q", task.StatusInProgress, taskToRun.Status)
			}
		})
	}
}

func TestDrudgerService_RunTask_CopiesBetweenTheCheckoutAndTheSetup(t *testing.T) {
	projectDir := setupProjectDir(t)
	writeFiles(t, projectDir, map[string]string{"api/.env": "TOKEN=1"})
	taskToRun := todoTask()
	taskToRun.Repositories = []string{"api"}
	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
	service := newTestServiceWith(settingsWithCopies(), commands, taskToRun)

	if err := service.RunTask(testProjectSlug, taskToRun.ID, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := []struct {
		step  string
		index int
	}{
		{step: "branch checkout", index: firstEventIndex[BranchCheckoutStarted](service.progress)},
		{step: "copy", index: firstEventIndex[LocalFileCopied](service.progress)},
		{step: "setup", index: firstEventIndex[SetupCommandStarted](service.progress)},
	}
	for index, current := range order {
		if current.index < 0 {
			t.Fatalf("expected the %s to be reported, got %+v", current.step, service.progress.events)
		}
		if index > 0 && current.index < order[index-1].index {
			t.Errorf("expected the %s after the %s, got %+v", current.step, order[index-1].step, service.progress.events)
		}
	}
}

func TestDrudgerService_RunTask_CopyFailureStopsTheLaunch(t *testing.T) {
	projectDir := setupProjectDir(t)
	writeFiles(t, projectDir, map[string]string{"api/.env": "TOKEN=1"})
	// A non-empty directory where the copy puts a file cannot be replaced.
	writeFiles(t, slotRoot(projectDir, 1), map[string]string{"api/.env/stale": "stale"})
	taskToRun := todoTask()
	taskToRun.Repositories = []string{"api"}
	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
	service := newTestServiceWith(settingsWithCopies(), commands, taskToRun)
	registerWrittenWorktrees(service, projectDir)

	err := service.RunTask(testProjectSlug, taskToRun.ID, false)

	if err == nil {
		t.Fatal("expected a failing copy to stop the launch")
	}
	for _, want := range []string{"api", ".env"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected the error to name %q, got %q", want, err)
		}
	}
	if index := firstEventIndex[SetupCommandStarted](service.progress); index >= 0 {
		t.Errorf("expected no setup to start, got %+v", service.progress.events)
	}
	if len(commands.started) != 0 {
		t.Errorf("expected no agent to start, got %q", commands.started)
	}
	if taskToRun.Status != task.StatusTodo {
		t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
	}
	if held := service.drudgers.holderOf(taskToRun.ID); held != nil {
		t.Errorf("expected the slot to be released, got Drudger %d holding the task", held.Slot)
	}
}

func TestDrudgerService_RunTask_DryRunListsTheCopies(t *testing.T) {
	cases := []struct {
		name         string
		repositories []string
		want         []LocalCopy
	}{
		{
			name: "a task naming no repositories lists the copies of all of them",
			want: []LocalCopy{{Repository: "api", Path: ".env"}, {Repository: "api", Path: "certs"}, {Repository: "ui", Path: ".env.local"}},
		},
		{
			name:         "a task lists only the copies of the repositories it changes",
			repositories: []string{"ui", "docs"},
			want:         []LocalCopy{{Repository: "ui", Path: ".env.local"}},
		},
		{
			name:         "a repository with no copies lists none",
			repositories: []string{"docs"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			writeFiles(t, projectDir, map[string]string{"api/.env": "TOKEN=1", "ui/.env.local": "PORT=3000"})
			taskToRun := todoTask()
			taskToRun.Repositories = testCase.repositories
			commands := &fakeCommandRunner{projectDir: projectDir}
			service := newTestServiceWith(settingsWithCopies(), commands, taskToRun)

			if err := service.RunTask(testProjectSlug, taskToRun.ID, true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			described := singleEvent[RunDescribed](t, service.progress)
			if !slices.Equal(described.Copies, testCase.want) {
				t.Errorf("expected the copies %+v, got %+v", testCase.want, described.Copies)
			}
			for _, repository := range []string{"api", "ui"} {
				if _, err := os.Stat(filepath.Join(slotRoot(projectDir, 1), repository)); !os.IsNotExist(err) {
					t.Errorf("expected no worktree of %s, got %v", repository, err)
				}
			}
			if reported := reportedEvents[LocalFileCopied](service.progress); len(reported) != 0 {
				t.Errorf("expected nothing copied, got %+v", reported)
			}
		})
	}
}
