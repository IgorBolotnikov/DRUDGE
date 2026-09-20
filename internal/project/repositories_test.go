package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/git"
)

// fakeGit answers the git port from what a test set up and counts the calls,
// so a test can assert that a question was never asked.
type fakeGit struct {
	roots    map[string]bool
	branches map[string]string
	calls    int
}

func (fake *fakeGit) IsRepositoryRoot(dir string) (bool, error) {
	fake.calls++
	return fake.roots[filepath.Clean(dir)], nil
}

func (fake *fakeGit) DefaultBranch(dir string) (string, error) {
	fake.calls++
	branch, isKnown := fake.branches[filepath.Clean(dir)]
	if !isKnown {
		return "", git.ErrNoDefaultBranch
	}
	return branch, nil
}

func (fake *fakeGit) HasRemote(dir string, remote string) (bool, error) {
	return false, fmt.Errorf("HasRemote should not be called")
}

func (fake *fakeGit) Fetch(dir string, remote string, branch string) error {
	return fmt.Errorf("Fetch should not be called")
}

func (fake *fakeGit) AddDetachedWorktree(dir string, path string, ref string) error {
	return fmt.Errorf("AddDetachedWorktree should not be called")
}

func (fake *fakeGit) RemoveWorktree(dir string, path string) error {
	return fmt.Errorf("RemoveWorktree should not be called")
}

func (fake *fakeGit) PruneWorktrees(dir string) error {
	return fmt.Errorf("PruneWorktrees should not be called")
}

func (fake *fakeGit) HasWorktree(dir string, path string) (bool, error) {
	return false, fmt.Errorf("HasWorktree should not be called")
}

func (fake *fakeGit) IsDirty(dir string) (bool, error) {
	return false, fmt.Errorf("IsDirty should not be called")
}

func (fake *fakeGit) Stash(dir string, message string) (string, error) {
	return "", fmt.Errorf("Stash should not be called")
}

func (fake *fakeGit) BranchExists(dir string, branch string) (bool, error) {
	return false, fmt.Errorf("BranchExists should not be called")
}

func (fake *fakeGit) CommitCount(dir string, base string, tip string) (int, error) {
	return 0, fmt.Errorf("CommitCount should not be called")
}

func (fake *fakeGit) CreateBranch(dir string, branch string, start string) error {
	return fmt.Errorf("CreateBranch should not be called")
}

func (fake *fakeGit) ResetBranch(dir string, branch string, start string) error {
	return fmt.Errorf("ResetBranch should not be called")
}

func (fake *fakeGit) DeleteBranch(dir string, branch string) error {
	return fmt.Errorf("DeleteBranch should not be called")
}

func (fake *fakeGit) CurrentBranch(dir string) (string, error) {
	return "", fmt.Errorf("CurrentBranch should not be called")
}

func (fake *fakeGit) BranchesContaining(dir string, commit string) ([]string, error) {
	return nil, fmt.Errorf("BranchesContaining should not be called")
}

func (fake *fakeGit) CheckoutDetached(dir string, ref string) error {
	return fmt.Errorf("CheckoutDetached should not be called")
}

func (fake *fakeGit) ResolveCommit(dir string, ref string) (git.Commit, error) {
	return git.Commit{}, fmt.Errorf("ResolveCommit should not be called")
}

func (fake *fakeGit) ResolveHeadCommit(dir string) (git.Commit, error) {
	return git.Commit{}, fmt.Errorf("ResolveHeadCommit should not be called")
}

// newFakeGit maps paths relative to projectDir onto what git would answer.
func newFakeGit(projectDir string, roots []string, branches map[string]string) *fakeGit {
	fake := &fakeGit{roots: map[string]bool{}, branches: map[string]string{}}
	for _, root := range roots {
		fake.roots[filepath.Join(projectDir, root)] = true
	}
	for path, branch := range branches {
		fake.branches[filepath.Join(projectDir, path)] = branch
	}
	return fake
}

func newTestService(gitOps git.Operations) *ProjectService {
	return NewProjectService(nil, gitOps, common.NewLogger(""))
}

func makeProjectDir(t *testing.T, subdirs []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, subdir := range subdirs {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0o755); err != nil {
			t.Fatalf("could not create %s: %v", subdir, err)
		}
	}
	return dir
}

func TestDiscoverRepositories(t *testing.T) {
	tests := []struct {
		name    string
		subdirs []string
		roots   []string
		want    []config.Repository
		wantErr bool
	}{
		{
			name:  "project directory is a repository",
			roots: []string{"."},
			want:  []config.Repository{{Path: "."}},
		},
		{
			name:    "project directory holds repositories",
			subdirs: []string{"api", "docs", "ui"},
			roots:   []string{"api", "ui"},
			want:    []config.Repository{{Path: "api"}, {Path: "ui"}},
		},
		{
			name:    "a repository wins over the subdirectories under it",
			subdirs: []string{"vendor"},
			roots:   []string{".", "vendor"},
			want:    []config.Repository{{Path: "."}},
		},
		{
			name:    "no repository anywhere",
			subdirs: []string{"docs"},
			wantErr: true,
		},
		{
			name:    "hidden subdirectory is ignored",
			subdirs: []string{".cache"},
			roots:   []string{".cache"},
			wantErr: true,
		},
		{
			name:    "empty directory",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectDir := makeProjectDir(t, test.subdirs)
			service := newTestService(newFakeGit(projectDir, test.roots, nil))

			got, err := service.DiscoverRepositories(projectDir)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				if !strings.Contains(err.Error(), projectDir) {
					t.Errorf("error = %q, want it to name %q", err, projectDir)
				}
				return
			}
			if err != nil {
				t.Fatalf("DiscoverRepositories: %v", err)
			}

			if len(got) != len(test.want) {
				t.Fatalf("DiscoverRepositories = %+v, want %+v", got, test.want)
			}
			for index, repository := range got {
				if repository != test.want[index] {
					t.Errorf("repository %d = %+v, want %+v", index, repository, test.want[index])
				}
			}
		})
	}
}

func TestDefaultBranch(t *testing.T) {
	tests := []struct {
		name           string
		repository     config.Repository
		roots          []string
		branches       map[string]string
		want           string
		wantErr        []string
		isGitLeftAlone bool
	}{
		{
			name:           "the config key wins",
			repository:     config.Repository{Path: "api", DefaultBranch: "trunk"},
			branches:       map[string]string{"api": "main"},
			want:           "trunk",
			isGitLeftAlone: true,
		},
		{
			name:       "origin/HEAD answers",
			repository: config.Repository{Path: "api"},
			roots:      []string{"api"},
			branches:   map[string]string{"api": "main"},
			want:       "main",
		},
		{
			name:       "neither answers",
			repository: config.Repository{Path: "api"},
			roots:      []string{"api"},
			wantErr:    []string{"git remote set-head origin -a", config.DefaultBranchKey, "api"},
		},
		{
			name:       "the path is not a repository",
			repository: config.Repository{Path: "api"},
			wantErr:    []string{"api", config.RepositoriesKey},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			fake := newFakeGit(projectDir, test.roots, test.branches)
			service := newTestService(fake)

			got, err := service.DefaultBranch(projectDir, test.repository)
			if test.isGitLeftAlone && fake.calls != 0 {
				t.Errorf("git was asked %d times, want it left alone", fake.calls)
			}
			if len(test.wantErr) > 0 {
				if err == nil {
					t.Fatal("expected an error")
				}
				for _, want := range test.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error = %q, want it to name %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("DefaultBranch: %v", err)
			}
			if got != test.want {
				t.Errorf("DefaultBranch = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveRepositories(t *testing.T) {
	want := []struct {
		path        string
		branch      string
		wantProblem bool
	}{
		{path: "api", branch: "main"},
		{path: "ui", wantProblem: true},
		{path: "docs", wantProblem: true},
	}

	projectDir := t.TempDir()
	service := newTestService(newFakeGit(projectDir, []string{"api", "ui"}, map[string]string{"api": "main"}))

	resolved := service.ResolveRepositories(projectDir, []config.Repository{{Path: "api"}, {Path: "ui"}, {Path: "docs"}})

	if len(resolved) != len(want) {
		t.Fatalf("ResolveRepositories returned %d entries, want %d", len(resolved), len(want))
	}
	for index, repository := range resolved {
		if repository.Repository.Path != want[index].path {
			t.Errorf("entry %d is %q, want %q", index, repository.Repository.Path, want[index].path)
		}
		if repository.DefaultBranch != want[index].branch {
			t.Errorf("%s resolved to %q, want %q", want[index].path, repository.DefaultBranch, want[index].branch)
		}
		if (repository.Problem != nil) != want[index].wantProblem {
			t.Errorf("%s problem = %v, want a problem: %v", want[index].path, repository.Problem, want[index].wantProblem)
		}
	}
}
