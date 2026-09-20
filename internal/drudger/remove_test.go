package drudger

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

func TestDrudgerService_RemoveRun(t *testing.T) {
	cases := []struct {
		name string
		// stream is what the run directory of the task holds. An empty one
		// stands for a task no agent has been given.
		stream  string
		wantRun bool
	}{
		{
			name:    "a task an agent has worked on",
			stream:  initEvent,
			wantRun: true,
		},
		{
			name: "a task no agent has been given",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRemove := todoTask()

			runDir := common.RunDir(projectDir, string(taskToRemove.ID))
			if testCase.stream != "" {
				writeStream(t, runDir, testCase.stream)
			}

			service := newTestService(taskToRemove)

			hasRun, err := service.RemoveRun(taskToRemove.ID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hasRun != testCase.wantRun {
				t.Errorf("expected a run directory to be reported: %v, got %v", testCase.wantRun, hasRun)
			}

			isLeft, err := common.Exists(runDir)
			if err != nil {
				t.Fatalf("could not check the run directory: %v", err)
			}
			if isLeft {
				t.Error("expected the run directory to be gone")
			}
		})
	}
}

func TestDrudgerService_RemoveEmptyBranches(t *testing.T) {
	cases := []struct {
		name string
		// repositories are the repositories of the project. An empty list
		// stands for the single repository most cases work with.
		repositories []string
		// holds is how many commits the branch of the task holds in each
		// repository, keyed by repository name.
		holds map[string]int

		wantDeletedIn []string
		// wantKeptIn names the repositories whose branch the output has to
		// name.
		wantKeptIn []string
	}{
		{
			name:       "a branch holding commits",
			holds:      map[string]int{testRepositoryName: 3},
			wantKeptIn: []string{testRepositoryName},
		},
		{
			name:          "a branch holding nothing",
			holds:         map[string]int{testRepositoryName: 0},
			wantDeletedIn: []string{testRepositoryName},
		},
		{
			name:          "one branch of two holding commits",
			repositories:  []string{"api", "ui"},
			holds:         map[string]int{"api": 2, "ui": 0},
			wantDeletedIn: []string{"ui"},
			wantKeptIn:    []string{"api"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			repositories := testCase.repositories
			if len(repositories) == 0 {
				repositories = []string{testRepositoryName}
			}

			removed := landedTask(repositories...)
			service := newTestServiceWithPool(localConfigWith(repositories...), config.DefaultConfig(), &fakeCommandRunner{}, nil, removed)

			dirs := repositoryDirsOf(projectDir, repositories)
			for repository, commits := range testCase.holds {
				service.git.branchHolding(dirs[repository], testTaskBranch, commits)
			}

			var err error
			output := captureOutput(func() {
				err = service.tasks.RemoveTask(testProjectSlug, removed.ID, true, service.DrudgerService, nil)
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantDeleted := make([]branchRef, 0, len(testCase.wantDeletedIn))
			for _, repository := range testCase.wantDeletedIn {
				wantDeleted = append(wantDeleted, branchRef{dir: dirs[repository], branch: testTaskBranch})
			}
			if !slices.Equal(service.git.deletedBranches, wantDeleted) {
				t.Errorf("expected the branches %v to be deleted, got %v", wantDeleted, service.git.deletedBranches)
			}

			for _, repository := range testCase.wantKeptIn {
				if !strings.Contains(output, testTaskBranch) || !strings.Contains(output, repository) {
					t.Errorf("expected the output to name branch %s of repository %s, got %q", testTaskBranch, repository, output)
				}
			}
			if _, lookupErr := service.taskRepo.GetTask(testProjectSlug, removed.ID); lookupErr == nil {
				t.Error("expected the task to be gone")
			}
		})
	}
}

func TestDrudgerService_RemoveEmptyBranches_ReadsNoGitForATaskThatNeverRan(t *testing.T) {
	setupProjectDir(t)

	removed := todoTask()
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), &fakeCommandRunner{}, nil, removed)
	service.gitOps = &refusingGit{t: t}

	var err error
	captureOutput(func() {
		err = service.tasks.RemoveTask(testProjectSlug, removed.ID, true, service.DrudgerService, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, lookupErr := service.taskRepo.GetTask(testProjectSlug, removed.ID); lookupErr == nil {
		t.Error("expected the task to be gone")
	}
}

func TestDrudgerService_RemoveEmptyBranches_ReportsABranchItCouldNotDelete(t *testing.T) {
	projectDir := setupProjectDir(t)

	removed := landedTask(testRepositoryName)
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), &fakeCommandRunner{}, nil, removed)

	dir := repositoryDirsOf(projectDir, []string{testRepositoryName})[testRepositoryName]
	service.git.branchHolding(dir, testTaskBranch, 0)
	// The fake git refuses to delete a branch a worktree has checked out.
	service.git.rememberBranch(dir, testTaskBranch)

	var err error
	var output string
	captureOutput(func() {
		output = captureErrors(func() {
			err = service.tasks.RemoveTask(testProjectSlug, removed.ID, true, service.DrudgerService, nil)
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, testTaskBranch) {
		t.Errorf("expected the branch that stayed to be named, got %q", output)
	}
	if _, lookupErr := service.taskRepo.GetTask(testProjectSlug, removed.ID); lookupErr == nil {
		t.Error("expected the task to be removed anyway")
	}
}

// landedTask is a task whose run is over, carrying the branch its work is on
// in each repository.
func landedTask(repositories ...string) *task.Task {
	landed := handedOverTask(repositories...)
	landed.Status = task.StatusDone
	return landed
}

// repositoryDirsOf maps each repository of a project to where it lives.
func repositoryDirsOf(projectDir string, repositories []string) map[string]string {
	dirs := make(map[string]string, len(repositories))
	for _, repository := range repositories {
		dirs[repository] = filepath.Join(projectDir, repository)
	}
	return dirs
}

// branchHolding gives one repository a branch holding a number of commits
// beyond the base it was cut from.
func (fake *fakeGit) branchHolding(dir string, branch string, commits int) {
	fake.registerBranch(dir, branch)
	if fake.repositoryCommits == nil {
		fake.repositoryCommits = map[string]int{}
	}
	fake.repositoryCommits[dir+" "+branch] = commits
}
