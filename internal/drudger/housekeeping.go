package drudger

import (
	"fmt"
	"strings"
	"unicode"

	"drudge/internal/git"
	"drudge/internal/task"
)

// branchPrefix groups every branch drudge makes under one name.
const branchPrefix = "drudge/"

// branchSlugLength caps how much of a task title a branch name carries.
const branchSlugLength = 40

// branchAttempts is how many names one task may take. Every name past the
// first belongs to a rerun that found the one before it holding commits.
const branchAttempts = 20

// shortSHALength is how much of a commit drudge prints.
const shortSHALength = 12

// handover is what housekeeping did to a workspace before an agent was given
// it.
type handover struct {
	// Branch is the branch every repository of the workspace is checked out
	// on. One name across the workspace keeps a change spanning two
	// repositories on one branch.
	Branch       string
	Repositories []repositoryHandover
}

// repositoryHandover is what housekeeping did in one repository.
type repositoryHandover struct {
	Name string
	// Stash is the commit holding what the last Session left uncommitted, and
	// is empty when the worktree was clean.
	Stash string
}

// prepareWorkspace puts every repository of a workspace into the state a
// handover needs. Whatever the last Session left uncommitted is stashed and
// the task's branch is checked out. It reports what it did.
//
// A step that fails stops the handover. An agent is never given a workspace
// that is not in the state it was meant to be.
func (service *DrudgerService) prepareWorkspace(space slotWorkspace, taskToRun *task.Task) (handover, error) {
	branch, err := service.pickTaskBranch(space, taskToRun)
	if err != nil {
		return handover{}, err
	}

	prepared := handover{Branch: branch, Repositories: make([]repositoryHandover, 0, len(space.Repositories))}
	for _, repository := range space.Repositories {
		stash, err := service.stashLeftovers(space.Slot, repository, taskToRun)
		if err != nil {
			return handover{}, err
		}

		if err := service.checkoutBranch(repository, branch); err != nil {
			return handover{}, err
		}

		prepared.Repositories = append(prepared.Repositories, repositoryHandover{Name: repository.Name, Stash: stash})
	}
	return prepared, nil
}

// stashLeftovers puts whatever the last Session left uncommitted in a worktree
// aside and returns the commit holding it. A clean worktree returns an empty
// commit. Untracked files go into the stash, so files the last agent never
// committed stay out of this task's branch.
func (service *DrudgerService) stashLeftovers(slot int, repository repositoryWorktree, taskToRun *task.Task) (string, error) {
	dirty, err := service.gitOps.IsDirty(repository.Worktree)
	if err != nil {
		return "", err
	}
	if !dirty {
		return "", nil
	}

	commit, err := service.gitOps.Stash(repository.Worktree, stashMessage(slot, taskToRun))
	if err != nil {
		return "", fmt.Errorf("could not stash what the last Session left in the workspace of repository %s: %w", repository.Name, err)
	}

	service.logger.Info("Repository %s held uncommitted changes, they are stashed at %s", repository.Name, shortSHA(commit))
	return commit, nil
}

// stashMessage says which slot made a stash and which task it was made for, so
// a stash list tells the user where an entry came from.
func stashMessage(slot int, taskToRun *task.Task) string {
	return fmt.Sprintf("drudge: slot %d before task %s %s", slot, task.ShortID(taskToRun.ID), taskToRun.Title)
}

// pickTaskBranch names the branch this handover puts on every repository of a
// workspace. A rerun is the same task, so it meets the branches its earlier
// attempts left. The name is the first in the series holding no work in any
// repository, which leaves every attempt that committed something reachable by
// its own branch.
func (service *DrudgerService) pickTaskBranch(space slotWorkspace, taskToRun *task.Task) (string, error) {
	wanted := branchFor(taskToRun)

	for attempt := 1; attempt <= branchAttempts; attempt++ {
		candidate := attemptBranch(wanted, attempt)

		free, err := service.branchHoldsNoWorkAnywhere(space, candidate)
		if err != nil {
			return "", err
		}
		if free {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(
		"task %s has %d branches holding commits already, merge or delete them and run this again",
		taskToRun.ID, branchAttempts,
	)
}

// branchHoldsNoWorkAnywhere reports whether a branch holds nothing any
// repository of a workspace does not already have on its base.
func (service *DrudgerService) branchHoldsNoWorkAnywhere(space slotWorkspace, branch string) (bool, error) {
	for _, repository := range space.Repositories {
		free, err := git.BranchHoldsNoWork(service.gitOps, repository.Dir, repository.BaseRef(), branch)
		if err != nil {
			return false, err
		}
		if !free {
			return false, nil
		}
	}
	return true, nil
}

// checkoutBranch checks a branch out in the worktree of a repository, cut from
// the base the repository works off. A branch that is already there is moved
// to that base, which the caller has established holds no work.
func (service *DrudgerService) checkoutBranch(repository repositoryWorktree, branch string) error {
	base := repository.BaseRef()

	exists, err := service.gitOps.BranchExists(repository.Dir, branch)
	if err != nil {
		return err
	}

	if exists {
		err = service.gitOps.ResetBranch(repository.Worktree, branch, base)
	} else {
		err = service.gitOps.CreateBranch(repository.Worktree, branch, base)
	}
	if err != nil {
		return fmt.Errorf("could not put branch %s on the workspace of repository %s: %w", branch, repository.Name, err)
	}
	return nil
}

// branchFor names the branch the work on a task goes on. It takes the whole
// task, because a branch name configurable per project is the next slice.
func branchFor(taskToRun *task.Task) string {
	// TODO: need to make it fully configurable and take the branch template
	// from the local config
	name := branchPrefix + task.ShortID(taskToRun.ID)
	if slug := branchSlug(taskToRun.Title); slug != "" {
		name += "-" + slug
	}
	return name
}

// attemptBranch names the nth branch of a task. The first attempt takes the
// plain name.
func attemptBranch(branch string, attempt int) string {
	if attempt == 1 {
		return branch
	}
	return fmt.Sprintf("%s-%d", branch, attempt)
}

// branchSlug folds a task title into the part of a branch name that comes from
// it. Everything git does not accept in a ref becomes a separator, and a title
// longer than branchSlugLength is cut at a word.
func branchSlug(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(letter rune) bool {
		return !unicode.IsLetter(letter) && !unicode.IsDigit(letter)
	})

	slug := []rune(strings.Join(words, "-"))
	if len(slug) <= branchSlugLength {
		return string(slug)
	}

	cut := string(slug[:branchSlugLength])
	if lastWord := strings.LastIndex(cut, "-"); lastWord > 0 {
		return cut[:lastWord]
	}
	return cut
}

// shortSHA cuts a commit down to what drudge prints.
func shortSHA(commit string) string {
	if len(commit) <= shortSHALength {
		return commit
	}
	return commit[:shortSHALength]
}
