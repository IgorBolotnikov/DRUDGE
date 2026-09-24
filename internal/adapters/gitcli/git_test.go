package gitcli

import (
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/exec"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
)

var testTimeouts = git.Timeouts{
	Fetch:    30 * time.Second,
	Worktree: 30 * time.Second,
	Command:  30 * time.Second,
}

// TestMain clears localEnvVars for the helpers that run git without the
// adapter. A git hook sets GIT_INDEX_FILE for every command it starts.
func TestMain(m *testing.M) {
	for _, name := range localEnvVars {
		os.Unsetenv(name)
	}
	os.Exit(m.Run())
}

func newTestAdapter() *Git {
	return New(exec.NewCommandRunner(), testTimeouts)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := osexec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, output)
	}
}

// initRepo creates a repository with one commit on branch and returns its path.
func initRepo(t *testing.T, dir string, branch string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("could not create %s: %v", dir, err)
	}
	runGit(t, dir, "init", "-q", "-b", branch)
	runGit(t, dir, "-c", "user.email=drudge@test", "-c", "user.name=drudge", "commit", "-q", "--allow-empty", "-m", "first")
	return dir
}

// cloneRepo clones a source repository on branch into dst and returns the
// clone. A clone is what writes origin/HEAD.
func cloneRepo(t *testing.T, root string, branch string, dst string) string {
	t.Helper()
	source := initRepo(t, filepath.Join(root, "source"), branch)
	clone := filepath.Join(root, dst)
	runGit(t, root, "clone", "-q", source, clone)
	return clone
}

func makeDir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("could not create %s: %v", path, err)
	}
	return path
}

func TestIsRepositoryRoot(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, root string) string
		want  bool
	}{
		{
			name:  "repository root",
			build: func(t *testing.T, root string) string { return initRepo(t, filepath.Join(root, "repo"), "main") },
			want:  true,
		},
		{
			name: "worktree of a repository",
			build: func(t *testing.T, root string) string {
				repo := initRepo(t, filepath.Join(root, "repo"), "main")
				worktree := filepath.Join(root, "worktree")
				runGit(t, repo, "worktree", "add", "--detach", worktree, "HEAD")
				return worktree
			},
			want: true,
		},
		{
			name:  "plain directory",
			build: func(t *testing.T, root string) string { return makeDir(t, filepath.Join(root, "plain")) },
			want:  false,
		},
		{
			name: "subdirectory of a repository",
			build: func(t *testing.T, root string) string {
				repo := initRepo(t, filepath.Join(root, "repo"), "main")
				return makeDir(t, filepath.Join(repo, "sub"))
			},
			want: false,
		},
		{
			name:  "missing directory",
			build: func(t *testing.T, root string) string { return filepath.Join(root, "gone") },
			want:  false,
		},
		{
			name: "bare repository",
			build: func(t *testing.T, root string) string {
				bare := makeDir(t, filepath.Join(root, "bare.git"))
				runGit(t, bare, "init", "-q", "--bare")
				return bare
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := test.build(t, t.TempDir())

			got, err := newTestAdapter().IsRepositoryRoot(dir)
			if err != nil {
				t.Fatalf("IsRepositoryRoot: %v", err)
			}
			if got != test.want {
				t.Errorf("IsRepositoryRoot(%s) = %v, want %v", dir, got, test.want)
			}
		})
	}
}

func TestDefaultBranch(t *testing.T) {
	tests := []struct {
		name    string
		build   func(t *testing.T, root string) string
		want    string
		wantErr error
	}{
		{
			name:  "clone of a repository on main",
			build: func(t *testing.T, root string) string { return cloneRepo(t, root, "main", "clone") },
			want:  "main",
		},
		{
			name:  "clone of a repository on trunk",
			build: func(t *testing.T, root string) string { return cloneRepo(t, root, "trunk", "clone") },
			want:  "trunk",
		},
		{
			name:    "repository with no remote",
			build:   func(t *testing.T, root string) string { return initRepo(t, filepath.Join(root, "repo"), "main") },
			wantErr: git.ErrNoDefaultBranch,
		},
		{
			name: "repository with a remote that has no origin/HEAD",
			build: func(t *testing.T, root string) string {
				clone := cloneRepo(t, root, "main", "clone")
				runGit(t, clone, "remote", "set-head", "origin", "--delete")
				return clone
			},
			wantErr: git.ErrNoDefaultBranch,
		},
		{
			name:    "plain directory",
			build:   func(t *testing.T, root string) string { return makeDir(t, filepath.Join(root, "plain")) },
			wantErr: git.ErrNoDefaultBranch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := test.build(t, t.TempDir())

			got, err := newTestAdapter().DefaultBranch(dir)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("DefaultBranch error = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("DefaultBranch: %v", err)
			}
			if got != test.want {
				t.Errorf("DefaultBranch(%s) = %q, want %q", dir, got, test.want)
			}
		})
	}
}

func TestHasRemote(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, root string) string
		want  bool
	}{
		{
			name:  "clone of a repository",
			build: func(t *testing.T, root string) string { return cloneRepo(t, root, "main", "clone") },
			want:  true,
		},
		{
			name:  "repository with no remote",
			build: func(t *testing.T, root string) string { return initRepo(t, filepath.Join(root, "repo"), "main") },
			want:  false,
		},
		{
			name: "repository with another remote",
			build: func(t *testing.T, root string) string {
				repo := initRepo(t, filepath.Join(root, "repo"), "main")
				runGit(t, repo, "remote", "add", "upstream", filepath.Join(root, "elsewhere"))
				return repo
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := test.build(t, t.TempDir())

			got, err := newTestAdapter().HasRemote(dir, git.OriginRemote)
			if err != nil {
				t.Fatalf("HasRemote: %v", err)
			}
			if got != test.want {
				t.Errorf("HasRemote(%s) = %v, want %v", dir, got, test.want)
			}
		})
	}
}

func TestFetch_MovesTheTrackingRef(t *testing.T) {
	root := t.TempDir()
	clone := cloneRepo(t, root, "main", "clone")
	source := filepath.Join(root, "source")
	runGit(t, source, "-c", "user.email=drudge@test", "-c", "user.name=drudge", "commit", "-q", "--allow-empty", "-m", "second")

	if err := newTestAdapter().Fetch(clone, git.OriginRemote, "main"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if got, want := revisionOf(t, clone, "refs/remotes/origin/main"), revisionOf(t, source, "refs/heads/main"); got != want {
		t.Errorf("origin/main is at %s, want %s", got, want)
	}
}

func TestFetch_UnreachableRemote(t *testing.T) {
	root := t.TempDir()
	clone := cloneRepo(t, root, "main", "clone")
	runGit(t, clone, "remote", "set-url", "origin", filepath.Join(root, "gone"))

	if err := newTestAdapter().Fetch(clone, git.OriginRemote, "main"); err == nil {
		t.Fatal("expected a fetch from a remote that is not there to fail")
	}
}

func TestAddDetachedWorktree(t *testing.T) {
	tests := []struct {
		name string
		ref  string
	}{
		{name: "a local branch", ref: "main"},
		{name: "a tracking ref", ref: "origin/main"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			clone := cloneRepo(t, root, "main", "clone")
			worktree := filepath.Join(root, "workspace", "slot-1")

			if err := newTestAdapter().AddDetachedWorktree(clone, worktree, test.ref); err != nil {
				t.Fatalf("AddDetachedWorktree: %v", err)
			}

			if got, want := revisionOf(t, worktree, "HEAD"), revisionOf(t, clone, test.ref); got != want {
				t.Errorf("the worktree is at %s, want %s", got, want)
			}
			// A worktree on a branch would keep that branch from being checked
			// out anywhere else, which is what an idle Drudger must not do.
			if branch := symbolicHead(t, worktree); branch != "" {
				t.Errorf("expected a detached HEAD, got branch %s", branch)
			}
		})
	}
}

func TestHasWorktree(t *testing.T) {
	tests := []struct {
		name string
		// build returns the path asked about.
		build func(t *testing.T, root string, clone string) string
		want  bool
	}{
		{
			name: "a worktree that was added",
			build: func(t *testing.T, root string, clone string) string {
				worktree := filepath.Join(root, "workspace", "slot-1")
				addWorktree(t, clone, worktree)
				return worktree
			},
			want: true,
		},
		{
			name: "a worktree whose directory was deleted",
			build: func(t *testing.T, root string, clone string) string {
				worktree := filepath.Join(root, "workspace", "slot-1")
				addWorktree(t, clone, worktree)
				if err := os.RemoveAll(worktree); err != nil {
					t.Fatalf("could not delete the worktree: %v", err)
				}
				return worktree
			},
			want: true,
		},
		{
			name:  "the main work tree of the repository",
			build: func(t *testing.T, root string, clone string) string { return clone },
			want:  true,
		},
		{
			name: "a plain directory at the path",
			build: func(t *testing.T, root string, clone string) string {
				return makeDir(t, filepath.Join(root, "workspace", "slot-1"))
			},
		},
		{
			name: "a path nothing was ever put at",
			build: func(t *testing.T, root string, clone string) string {
				return filepath.Join(root, "workspace", "slot-1")
			},
		},
		{
			name: "a worktree of another repository",
			build: func(t *testing.T, root string, clone string) string {
				other := initRepo(t, filepath.Join(root, "other"), "main")
				worktree := filepath.Join(root, "workspace", "slot-1")
				addWorktree(t, other, worktree)
				return worktree
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			clone := cloneRepo(t, root, "main", "clone")
			path := test.build(t, root, clone)

			got, err := newTestAdapter().HasWorktree(clone, path)
			if err != nil {
				t.Fatalf("HasWorktree: %v", err)
			}
			if got != test.want {
				t.Errorf("expected %v, got %v", test.want, got)
			}
		})
	}
}

// addWorktree checks a repository out in a detached worktree at path.
func addWorktree(t *testing.T, repo string, path string) {
	t.Helper()
	runGit(t, repo, "worktree", "add", "--detach", path, "HEAD")
}

func TestGit_IgnoresTheRepositoryTheCallerPointsAt(t *testing.T) {
	tests := []struct {
		name string
		// environment returns the variables the caller of drudge has set.
		// foreign is a repository drudge was not asked about.
		environment func(foreign string) map[string]string
	}{
		{
			name:        "a git hook of a plain commit",
			environment: func(foreign string) map[string]string { return map[string]string{"GIT_INDEX_FILE": ".git/index"} },
		},
		{
			name: "a git hook of a commit -a",
			environment: func(foreign string) map[string]string {
				return map[string]string{"GIT_INDEX_FILE": filepath.Join(foreign, ".git", "index.lock")}
			},
		},
		{
			name: "a repository named outright",
			environment: func(foreign string) map[string]string {
				return map[string]string{"GIT_DIR": filepath.Join(foreign, ".git"), "GIT_WORK_TREE": foreign}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			clone := cloneRepo(t, root, "main", "clone")
			foreign := initRepo(t, filepath.Join(root, "foreign"), "main")
			worktree := filepath.Join(root, "workspace", "slot-1")

			for name, value := range test.environment(foreign) {
				t.Setenv(name, value)
			}

			if err := newTestAdapter().AddDetachedWorktree(clone, worktree, "main"); err != nil {
				t.Fatalf("AddDetachedWorktree: %v", err)
			}
			if _, err := os.Stat(filepath.Join(foreign, ".git", "index.lock")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("expected the foreign repository to be left alone, got an index lock there: %v", err)
			}

			isRoot, err := newTestAdapter().IsRepositoryRoot(worktree)
			if err != nil {
				t.Fatalf("IsRepositoryRoot: %v", err)
			}
			if !isRoot {
				t.Errorf("expected %s to be the root of a work tree", worktree)
			}
		})
	}
}

func TestAddDetachedWorktree_PathHoldingFiles(t *testing.T) {
	root := t.TempDir()
	clone := cloneRepo(t, root, "main", "clone")
	taken := makeDir(t, filepath.Join(root, "taken"))
	if err := os.WriteFile(filepath.Join(taken, "leftover.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("could not write the leftover file: %v", err)
	}

	if err := newTestAdapter().AddDetachedWorktree(clone, taken, "main"); err == nil {
		t.Fatal("expected a worktree over a directory holding files to fail")
	}
}

// revisionOf returns the commit a ref points at.
func revisionOf(t *testing.T, dir string, ref string) string {
	t.Helper()
	command := osexec.Command("git", "rev-parse", ref)
	command.Dir = dir
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s in %s: %v", ref, dir, err)
	}
	return strings.TrimSpace(string(out))
}

// symbolicHead returns the branch a work tree has checked out, and an empty
// string when its HEAD is detached.
func symbolicHead(t *testing.T, dir string) string {
	t.Helper()
	command := osexec.Command("git", "symbolic-ref", "--short", "-q", "HEAD")
	command.Dir = dir
	out, _ := command.Output()
	return strings.TrimSpace(string(out))
}

func TestIsDirty(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, repo string)
		want  bool
	}{
		{
			name:  "a work tree holding nothing new",
			build: func(t *testing.T, repo string) {},
			want:  false,
		},
		{
			name: "a tracked file that changed",
			build: func(t *testing.T, repo string) {
				commitFile(t, repo, "main.go", "package main")
				writeFile(t, repo, "main.go", "package other")
			},
			want: true,
		},
		{
			name:  "a file git has never seen",
			build: func(t *testing.T, repo string) { writeFile(t, repo, "notes.txt", "scratch") },
			want:  true,
		},
		{
			name: "a file the repository ignores",
			build: func(t *testing.T, repo string) {
				commitFile(t, repo, ".gitignore", "build/\n")
				makeDir(t, filepath.Join(repo, "build"))
				writeFile(t, repo, "build/cache", "bytes")
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")
			test.build(t, repo)

			got, err := newTestAdapter().IsDirty(repo)
			if err != nil {
				t.Fatalf("IsDirty: %v", err)
			}
			if got != test.want {
				t.Errorf("IsDirty(%s) = %v, want %v", repo, got, test.want)
			}
		})
	}
}

func TestStash_TakesEverythingTheAgentLeft(t *testing.T) {
	repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")
	commitFile(t, repo, ".gitignore", "build/\n")
	commitFile(t, repo, "main.go", "package main")
	writeFile(t, repo, "main.go", "package changed")
	writeFile(t, repo, "notes.txt", "scratch")
	makeDir(t, filepath.Join(repo, "build"))
	writeFile(t, repo, "build/cache", "bytes")

	adapter := newTestAdapter()
	commit, err := adapter.Stash(repo, "drudge: slot 1")
	if err != nil {
		t.Fatalf("Stash: %v", err)
	}

	if commit != revisionOf(t, repo, "refs/stash") {
		t.Errorf("expected the commit of the stash, got %q", commit)
	}
	isDirty, err := adapter.IsDirty(repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if isDirty {
		t.Error("expected the work tree to be clean after the stash")
	}
	if _, err := os.Stat(filepath.Join(repo, "notes.txt")); !os.IsNotExist(err) {
		t.Error("expected the untracked file to go into the stash")
	}
	if _, err := os.Stat(filepath.Join(repo, "build", "cache")); err != nil {
		t.Errorf("expected the ignored file to stay: %v", err)
	}
	if list := gitOutput(t, repo, "stash", "list"); !strings.Contains(list, "drudge: slot 1") {
		t.Errorf("expected the stash to carry its message, got %q", list)
	}
}

func TestStash_CleanWorkTree(t *testing.T) {
	repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")
	commitFile(t, repo, "main.go", "package main")
	writeFile(t, repo, "main.go", "package changed")

	adapter := newTestAdapter()
	if _, err := adapter.Stash(repo, "drudge: the first stash"); err != nil {
		t.Fatalf("Stash: %v", err)
	}

	commit, err := adapter.Stash(repo, "drudge: nothing to take")
	if err != nil {
		t.Fatalf("Stash: %v", err)
	}
	if commit != "" {
		t.Errorf("expected a clean work tree to be stashed as nothing, got %q", commit)
	}
}

func TestStash_InAWorktree(t *testing.T) {
	root := t.TempDir()
	repo := initRepo(t, filepath.Join(root, "repo"), "main")
	commitFile(t, repo, "main.go", "package main")
	worktree := filepath.Join(root, "slot-1")
	runGit(t, repo, "worktree", "add", "--detach", worktree, "main")
	writeFile(t, worktree, "main.go", "package changed")

	commit, err := newTestAdapter().Stash(worktree, "drudge: slot 1")
	if err != nil {
		t.Fatalf("Stash: %v", err)
	}

	// One stash list is shared by every worktree of a repository, so what a
	// slot stashes is what the user sees in their own checkout.
	if commit != revisionOf(t, repo, "refs/stash") {
		t.Errorf("expected the stash of the worktree in the repository, got %q", commit)
	}
}

func TestBranchExists(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   bool
	}{
		{name: "a branch the repository has", branch: "main", want: true},
		{name: "a branch nobody made", branch: "drudge/task-1-fix-login", want: false},
		{name: "a tag of the same name as no branch", branch: "v1", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")
			runGit(t, repo, "tag", "v1")

			got, err := newTestAdapter().BranchExists(repo, test.branch)
			if err != nil {
				t.Fatalf("BranchExists: %v", err)
			}
			if got != test.want {
				t.Errorf("BranchExists(%s) = %v, want %v", test.branch, got, test.want)
			}
		})
	}
}

func TestCommitCount(t *testing.T) {
	tests := []struct {
		name string
		// commits is how many commits are made on the branch after it is cut
		// from main.
		commits int
		// shouldMoveBase says whether main moves on after the branch was cut.
		shouldMoveBase bool
		want           int
	}{
		{name: "a branch holding nothing of its own", commits: 0, want: 0},
		{name: "a branch holding two commits", commits: 2, want: 2},
		{name: "a branch left behind by the default branch", commits: 0, shouldMoveBase: true, want: 0},
		{name: "a branch holding one commit while the default branch moved", commits: 1, shouldMoveBase: true, want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")
			runGit(t, repo, "switch", "-c", "drudge/task-1", "main")
			for index := range test.commits {
				commitFile(t, repo, fmt.Sprintf("work-%d.txt", index), "work")
			}
			runGit(t, repo, "switch", "main")
			if test.shouldMoveBase {
				commitFile(t, repo, "other.txt", "other")
			}

			got, err := newTestAdapter().CommitCount(repo, "main", "drudge/task-1")
			if err != nil {
				t.Fatalf("CommitCount: %v", err)
			}
			if got != test.want {
				t.Errorf("CommitCount = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCommitCount_UnknownRef(t *testing.T) {
	repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")

	if _, err := newTestAdapter().CommitCount(repo, "main", "drudge/nothing"); err == nil {
		t.Fatal("expected counting the commits of a branch that is not there to fail")
	}
}

func TestCreateBranch(t *testing.T) {
	root := t.TempDir()
	clone := cloneRepo(t, root, "main", "clone")
	worktree := filepath.Join(root, "slot-1")
	runGit(t, clone, "worktree", "add", "--detach", worktree, "origin/main")

	adapter := newTestAdapter()
	// The default branch is checked out in the clone, and a worktree may still
	// cut a branch from it.
	if err := adapter.CreateBranch(worktree, "drudge/task-1", "origin/main"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	if branch := symbolicHead(t, worktree); branch != "drudge/task-1" {
		t.Errorf("expected the worktree on drudge/task-1, got %q", branch)
	}
	if got, want := revisionOf(t, worktree, "HEAD"), revisionOf(t, clone, "origin/main"); got != want {
		t.Errorf("the branch is at %s, want %s", got, want)
	}

	if err := adapter.CreateBranch(worktree, "drudge/task-1", "origin/main"); err == nil {
		t.Fatal("expected creating a branch that is already there to fail")
	}
}

func TestResetBranch(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, clone string, worktree string)
	}{
		{
			name:  "a branch nobody made yet",
			build: func(t *testing.T, clone string, worktree string) {},
		},
		{
			name: "a branch an earlier attempt left",
			build: func(t *testing.T, clone string, worktree string) {
				runGit(t, clone, "branch", "drudge/task-1", "origin/main")
			},
		},
		{
			name: "the branch the worktree is already on",
			build: func(t *testing.T, clone string, worktree string) {
				runGit(t, worktree, "switch", "-c", "drudge/task-1", "origin/main")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			clone := cloneRepo(t, root, "main", "clone")
			worktree := filepath.Join(root, "slot-1")
			runGit(t, clone, "worktree", "add", "--detach", worktree, "origin/main")
			test.build(t, clone, worktree)
			// The clone moves on, so resetting the branch to main has to move it.
			commitFile(t, clone, "moved-on.txt", "work")

			if err := newTestAdapter().ResetBranch(worktree, "drudge/task-1", "main"); err != nil {
				t.Fatalf("ResetBranch: %v", err)
			}

			if branch := symbolicHead(t, worktree); branch != "drudge/task-1" {
				t.Errorf("expected the worktree on drudge/task-1, got %q", branch)
			}
			if got, want := revisionOf(t, worktree, "HEAD"), revisionOf(t, clone, "main"); got != want {
				t.Errorf("the branch is at %s, want %s", got, want)
			}
		})
	}
}

func TestResolveCommit(t *testing.T) {
	repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")

	commit, err := newTestAdapter().ResolveCommit(repo, "main")
	if err != nil {
		t.Fatalf("ResolveCommit: %v", err)
	}

	if commit.SHA != revisionOf(t, repo, "main") {
		t.Errorf("expected the commit main points at, got %q", commit.SHA)
	}
	if elapsed := time.Since(commit.CommittedAt); elapsed < 0 || elapsed > time.Hour {
		t.Errorf("expected the commit to have just been made, got %s", commit.CommittedAt)
	}
}

func TestResolveCommit_UnknownRef(t *testing.T) {
	repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")

	if _, err := newTestAdapter().ResolveCommit(repo, "origin/main"); err == nil {
		t.Fatal("expected resolving a ref that is not there to fail")
	}
}

func TestResolveHeadCommit(t *testing.T) {
	tests := []struct {
		name string
		// build makes the repository HEAD is read in and returns its path
		// together with the commit the adapter is expected to report.
		build   func(t *testing.T, root string) (dir string, want string)
		wantErr bool
	}{
		{
			name: "a repository on its default branch",
			build: func(t *testing.T, root string) (string, string) {
				repo := initRepo(t, filepath.Join(root, "repo"), "main")
				return repo, revisionOf(t, repo, "main")
			},
		},
		{
			name: "a repository on a branch of its own",
			build: func(t *testing.T, root string) (string, string) {
				repo := initRepo(t, filepath.Join(root, "repo"), "main")
				runGit(t, repo, "switch", "-q", "-c", "drudge/task-1")
				commitFile(t, repo, "work.txt", "work")
				return repo, revisionOf(t, repo, "drudge/task-1")
			},
		},
		{
			name: "a worktree on a branch the repository is not on",
			build: func(t *testing.T, root string) (string, string) {
				repo := initRepo(t, filepath.Join(root, "repo"), "main")
				worktree := filepath.Join(root, "slot-1")
				addWorktree(t, repo, worktree)
				runGit(t, worktree, "switch", "-q", "-c", "drudge/task-1")
				commitFile(t, worktree, "work.txt", "work")
				return worktree, revisionOf(t, worktree, "drudge/task-1")
			},
		},
		{
			name: "a detached HEAD",
			build: func(t *testing.T, root string) (string, string) {
				repo := initRepo(t, filepath.Join(root, "repo"), "main")
				commitFile(t, repo, "work.txt", "work")
				detached := revisionOf(t, repo, "HEAD")
				runGit(t, repo, "checkout", "-q", "--detach", detached)
				return repo, detached
			},
		},
		{
			name: "a repository with no commits",
			build: func(t *testing.T, root string) (string, string) {
				repo := makeDir(t, filepath.Join(root, "repo"))
				runGit(t, repo, "init", "-q", "-b", "main")
				return repo, ""
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir, want := test.build(t, t.TempDir())

			commit, err := newTestAdapter().ResolveHeadCommit(dir)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected reading HEAD in %s to fail, got %s", dir, commit.SHA)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveHeadCommit: %v", err)
			}

			if commit.SHA != want {
				t.Errorf("HEAD is at %s, want %s", commit.SHA, want)
			}
			if elapsed := time.Since(commit.CommittedAt); elapsed < 0 || elapsed > time.Hour {
				t.Errorf("expected the commit to have just been made, got %s", commit.CommittedAt)
			}
		})
	}
}

// writeFile writes a file of a repository, creating none of its parents.
func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("could not write %s in %s: %v", name, dir, err)
	}
}

// commitFile writes a file of a repository and commits it.
func commitFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	writeFile(t, dir, name, content)
	runGit(t, dir, "add", name)
	runGit(t, dir, "-c", "user.email=drudge@test", "-c", "user.name=drudge", "commit", "-q", "-m", "add "+name)
}

// gitOutput returns what a git command printed.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := osexec.Command("git", args...)
	command.Dir = dir
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return string(out)
}

func TestDeleteBranch(t *testing.T) {
	tests := []struct {
		name string
		// build puts the branch in the repository and returns the directory
		// the deletion is asked for.
		build   func(t *testing.T, root string, repo string) string
		wantErr bool
	}{
		{
			name: "a branch nothing has checked out",
			build: func(t *testing.T, root string, repo string) string {
				runGit(t, repo, "branch", "drudge/task-1", "main")
				return repo
			},
		},
		{
			name: "a branch holding commits",
			build: func(t *testing.T, root string, repo string) string {
				runGit(t, repo, "branch", "drudge/task-1", "main")
				worktree := filepath.Join(root, "slot-1")
				runGit(t, repo, "worktree", "add", "-q", worktree, "drudge/task-1")
				commitFile(t, worktree, "work.txt", "work")
				runGit(t, repo, "worktree", "remove", "--force", worktree)
				return repo
			},
		},
		{
			name: "a branch a worktree is on",
			build: func(t *testing.T, root string, repo string) string {
				worktree := filepath.Join(root, "slot-1")
				runGit(t, repo, "worktree", "add", "-q", "-b", "drudge/task-1", worktree, "main")
				return repo
			},
			wantErr: true,
		},
		{
			name: "a branch the repository does not have",
			build: func(t *testing.T, root string, repo string) string {
				return repo
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			repo := initRepo(t, filepath.Join(root, "repo"), "main")
			dir := test.build(t, root, repo)

			err := newTestAdapter().DeleteBranch(dir, "drudge/task-1")

			if test.wantErr {
				if err == nil {
					t.Fatal("expected deleting the branch to fail")
				}
				return
			}
			if err != nil {
				t.Fatalf("DeleteBranch: %v", err)
			}
			if isPresent, _ := newTestAdapter().BranchExists(repo, "drudge/task-1"); isPresent {
				t.Error("expected the branch to be gone")
			}
		})
	}
}

func TestCurrentBranch(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, repo string)
		want  string
	}{
		{
			name:  "a work tree on a branch",
			build: func(t *testing.T, repo string) {},
			want:  "main",
		},
		{
			name: "a work tree on a branch drudge made",
			build: func(t *testing.T, repo string) {
				runGit(t, repo, "switch", "-q", "-c", "drudge/task-1")
			},
			want: "drudge/task-1",
		},
		{
			name: "a detached HEAD",
			build: func(t *testing.T, repo string) {
				runGit(t, repo, "switch", "-q", "--detach", "main")
			},
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")
			test.build(t, repo)

			got, err := newTestAdapter().CurrentBranch(repo)
			if err != nil {
				t.Fatalf("CurrentBranch: %v", err)
			}
			if got != test.want {
				t.Errorf("CurrentBranch = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBranchesContaining(t *testing.T) {
	tests := []struct {
		name string
		// build makes the commit the branches are read for and returns it.
		build func(t *testing.T, repo string) string
		want  []string
	}{
		{
			name: "a commit on one branch",
			build: func(t *testing.T, repo string) string {
				runGit(t, repo, "switch", "-q", "-c", "drudge/task-1")
				commitFile(t, repo, "work.txt", "work")
				runGit(t, repo, "switch", "-q", "main")
				return revisionOf(t, repo, "drudge/task-1")
			},
			want: []string{"drudge/task-1"},
		},
		{
			name: "a commit every branch reaches",
			build: func(t *testing.T, repo string) string {
				runGit(t, repo, "branch", "drudge/task-1", "main")
				return revisionOf(t, repo, "main")
			},
			want: []string{"drudge/task-1", "main"},
		},
		{
			name: "a commit no branch reaches",
			build: func(t *testing.T, repo string) string {
				runGit(t, repo, "switch", "-q", "--detach", "main")
				commitFile(t, repo, "work.txt", "work")
				return revisionOf(t, repo, "HEAD")
			},
			want: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t, filepath.Join(t.TempDir(), "repo"), "main")
			commit := test.build(t, repo)

			got, err := newTestAdapter().BranchesContaining(repo, commit)
			if err != nil {
				t.Fatalf("BranchesContaining: %v", err)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("BranchesContaining = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCheckoutDetached(t *testing.T) {
	root := t.TempDir()
	repo := initRepo(t, filepath.Join(root, "repo"), "main")
	worktree := filepath.Join(root, "slot-1")
	runGit(t, repo, "worktree", "add", "-q", "-b", "drudge/task-1", worktree, "main")

	if err := newTestAdapter().CheckoutDetached(worktree, revisionOf(t, worktree, "HEAD")); err != nil {
		t.Fatalf("CheckoutDetached: %v", err)
	}

	if branch := symbolicHead(t, worktree); branch != "" {
		t.Errorf("expected a detached HEAD, got branch %q", branch)
	}
	// The branch is free once no work tree holds it.
	if err := newTestAdapter().DeleteBranch(repo, "drudge/task-1"); err != nil {
		t.Errorf("expected the branch to be deletable after detaching: %v", err)
	}
}

func TestRemoveWorktree(t *testing.T) {
	tests := []struct {
		name string
		// leave puts the worktree in the state the removal finds it in.
		leave func(t *testing.T, worktree string)
	}{
		{
			name:  "a clean worktree",
			leave: func(t *testing.T, worktree string) {},
		},
		{
			name: "a worktree holding uncommitted changes",
			leave: func(t *testing.T, worktree string) {
				writeFile(t, worktree, "tracked.txt", "edited")
				runGit(t, worktree, "add", "tracked.txt")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			clone := cloneRepo(t, root, "main", "clone")
			worktree := filepath.Join(root, "workspace", "slot-1")
			addWorktree(t, clone, worktree)
			test.leave(t, worktree)

			adapter := newTestAdapter()
			if err := adapter.RemoveWorktree(clone, worktree); err != nil {
				t.Fatalf("RemoveWorktree: %v", err)
			}

			if _, err := os.Stat(worktree); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("expected the worktree directory to be gone, got %v", err)
			}
			isKnown, err := adapter.HasWorktree(clone, worktree)
			if err != nil {
				t.Fatalf("HasWorktree: %v", err)
			}
			if isKnown {
				t.Error("expected the repository to forget the worktree")
			}
		})
	}
}

func TestRemoveWorktree_APathThatIsNotAWorktree(t *testing.T) {
	root := t.TempDir()
	clone := cloneRepo(t, root, "main", "clone")

	if err := newTestAdapter().RemoveWorktree(clone, filepath.Join(root, "workspace", "slot-1")); err == nil {
		t.Fatal("expected removing a path the repository does not know to fail")
	}
}

func TestPruneWorktrees(t *testing.T) {
	tests := []struct {
		name string
		// build returns the path the repository is asked about afterwards.
		build func(t *testing.T, root string, clone string) string
		// wantKnown says whether the repository still knows the path once the
		// prune is over.
		wantKnown bool
	}{
		{
			name: "a worktree whose directory was deleted",
			build: func(t *testing.T, root string, clone string) string {
				worktree := filepath.Join(root, "workspace", "slot-1")
				addWorktree(t, clone, worktree)
				if err := os.RemoveAll(worktree); err != nil {
					t.Fatalf("could not delete the worktree: %v", err)
				}
				return worktree
			},
		},
		{
			name: "a worktree that is still there",
			build: func(t *testing.T, root string, clone string) string {
				worktree := filepath.Join(root, "workspace", "slot-1")
				addWorktree(t, clone, worktree)
				return worktree
			},
			wantKnown: true,
		},
		{
			name: "a repository with no worktrees to prune",
			build: func(t *testing.T, root string, clone string) string {
				return filepath.Join(root, "workspace", "slot-1")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			clone := cloneRepo(t, root, "main", "clone")
			path := test.build(t, root, clone)

			adapter := newTestAdapter()
			if err := adapter.PruneWorktrees(clone); err != nil {
				t.Fatalf("PruneWorktrees: %v", err)
			}

			isKnown, err := adapter.HasWorktree(clone, path)
			if err != nil {
				t.Fatalf("HasWorktree: %v", err)
			}
			if isKnown != test.wantKnown {
				t.Errorf("expected the repository to know %s: %v, got %v", path, test.wantKnown, isKnown)
			}
		})
	}
}
