package gitcli

import (
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"drudge/internal/adapters/exec"
	"drudge/internal/git"
)

var testTimeouts = git.Timeouts{
	Fetch:    30 * time.Second,
	Worktree: 30 * time.Second,
	Command:  30 * time.Second,
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
