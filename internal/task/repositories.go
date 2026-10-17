package task

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// validateRepositoryEdit refuses an edit that changes the repositories in more
// than one way, or that adds or removes an empty list.
func validateRepositoryEdit(changes EditTaskDto) error {
	var named []string
	if changes.Repositories != nil {
		named = append(named, "the repository list")
	}
	if changes.AddRepositories != nil {
		named = append(named, "the repositories to add")
	}
	if changes.RemoveRepositories != nil {
		named = append(named, "the repositories to remove")
	}
	if len(named) > 1 {
		return fmt.Errorf("an edit changes the repositories one way at a time, got %s", common.JoinNames(named))
	}

	if changes.AddRepositories != nil && len(*changes.AddRepositories) == 0 {
		return errors.New("name at least one repository to add")
	}
	if changes.RemoveRepositories != nil && len(*changes.RemoveRepositories) == 0 {
		return errors.New("name at least one repository to remove")
	}
	return nil
}

// resolveRepositories returns the paths with repeats dropped. It refuses a
// path the project does not record, and lists the paths it does.
func (service *TaskService) resolveRepositories(paths []string) ([]string, error) {
	var resolved []string
	for _, path := range paths {
		if !slices.Contains(service.repositories, path) {
			return nil, unknownRepositoryError(path, service.repositories)
		}
		if !slices.Contains(resolved, path) {
			resolved = append(resolved, path)
		}
	}
	return resolved, nil
}

// unknownRepositoryError names a path the project does not record.
func unknownRepositoryError(path string, known []string) error {
	if len(known) == 0 {
		return fmt.Errorf("the project has no repository %q, it records no repositories at all", path)
	}
	return fmt.Errorf("the project has no repository %q, it has: %s", path, strings.Join(known, ", "))
}

// refuseRepositoryRemoval refuses removing a path the task does not list, and
// removing every path it lists. A task that lists no repository works on all
// of them, so removing the last one would widen the task.
func refuseRepositoryRemoval(taskToEdit *Task, removed []string) error {
	for _, path := range removed {
		if !slices.Contains(taskToEdit.Repositories, path) {
			return fmt.Errorf("task %s does not list repository %q", ShortID(taskToEdit.ID), path)
		}
	}
	if len(removeRepositories(taskToEdit.Repositories, removed)) == 0 {
		return fmt.Errorf("removing every repository of task %s leaves it on all repositories of the project, clear the list to mean that", ShortID(taskToEdit.ID))
	}
	return nil
}

// addRepositories appends the paths repositories does not hold yet.
func addRepositories(repositories []string, added []string) []string {
	for _, path := range added {
		if !slices.Contains(repositories, path) {
			repositories = append(repositories, path)
		}
	}
	return repositories
}

// removeRepositories returns repositories without the removed paths, and nil
// when none are left.
func removeRepositories(repositories []string, removed []string) []string {
	var kept []string
	for _, path := range repositories {
		if !slices.Contains(removed, path) {
			kept = append(kept, path)
		}
	}
	return kept
}

// ChangesRepository reports whether a task works on the repository at a path.
// A task that lists no repository works on all of them.
func (taskToRead *Task) ChangesRepository(path string) bool {
	return len(taskToRead.Repositories) == 0 || slices.Contains(taskToRead.Repositories, path)
}
