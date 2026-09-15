package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"drudge/internal/config"
	"drudge/internal/git"
)

// selfPath is the recorded path of a project directory that is itself a
// repository.
const selfPath = "."

// hiddenPrefix marks the directories a scan skips. Drudge keeps its own
// worktrees and runs under one of them.
const hiddenPrefix = "."

// DiscoverRepositories works out the git repositories of a project directory.
// A directory that is itself a repository is the single entry ".". Otherwise
// every immediate subdirectory that is a repository becomes one entry, as a
// path relative to the project directory. A directory that is neither fails.
func (service *ProjectService) DiscoverRepositories(projectDir string) ([]config.Repository, error) {
	isRepository, err := service.gitOps.IsRepositoryRoot(projectDir)
	if err != nil {
		return nil, err
	}
	if isRepository {
		return []config.Repository{{Path: selfPath}}, nil
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", projectDir, err)
	}

	repositories := make([]config.Repository, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), hiddenPrefix) {
			continue
		}
		isRepository, err := service.gitOps.IsRepositoryRoot(filepath.Join(projectDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if isRepository {
			repositories = append(repositories, config.Repository{Path: entry.Name()})
		}
	}

	if len(repositories) == 0 {
		return nil, fmt.Errorf("drudge runs agents in git worktrees, and %s is not a git repository and holds none", projectDir)
	}
	return repositories, nil
}

// ResolvedRepository is a repository with the branch it cuts work from. A
// repository whose default branch does not resolve carries the reason in
// Problem and an empty DefaultBranch.
type ResolvedRepository struct {
	Repository    config.Repository
	DefaultBranch string
	Problem       error
}

// ResolveRepositories works out the default branch of every repository. One
// repository that does not resolve carries its reason and the others are still
// answered, so a listing shows the whole project.
func (service *ProjectService) ResolveRepositories(projectDir string, repositories []config.Repository) []ResolvedRepository {
	resolved := make([]ResolvedRepository, 0, len(repositories))
	for _, repository := range repositories {
		branch, err := service.DefaultBranch(projectDir, repository)
		resolved = append(resolved, ResolvedRepository{
			Repository:    repository,
			DefaultBranch: branch,
			Problem:       err,
		})
	}
	return resolved
}

// DefaultBranch works out the branch a repository's work is cut from. The
// defaultBranch key of the repository wins, and git is left alone when it is
// set. Otherwise the branch comes from origin/HEAD. A repository that answers
// neither fails with both fixes named.
func (service *ProjectService) DefaultBranch(projectDir string, repository config.Repository) (string, error) {
	if repository.DefaultBranch != "" {
		return repository.DefaultBranch, nil
	}

	dir := filepath.Join(projectDir, repository.Path)
	isRepository, err := service.gitOps.IsRepositoryRoot(dir)
	if err != nil {
		return "", err
	}
	if !isRepository {
		return "", fmt.Errorf("%s is not a git repository, fix the %q entry for %q in the local config", dir, config.RepositoriesKey, repository.Path)
	}

	branch, err := service.gitOps.DefaultBranch(dir)
	if errors.Is(err, git.ErrNoDefaultBranch) {
		return "", fmt.Errorf(
			"could not work out the default branch of %s, run `git remote set-head origin -a` in it, or set %q for it in the local config",
			repository.Path, config.DefaultBranchKey,
		)
	}
	if err != nil {
		return "", err
	}
	return branch, nil
}
