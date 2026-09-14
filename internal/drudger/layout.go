package drudger

import (
	"fmt"

	"drudge/internal/common"
	"drudge/internal/task"
)

// projectLayout is where a project lives on disk and where the files of its
// runs go.
type projectLayout struct {
	// Dir is the directory drudge was invoked in.
	Dir string
}

// RunDir returns the directory the run files of a task live in.
func (layout projectLayout) RunDir(taskID task.TaskID) string {
	return common.RunDir(layout.Dir, string(taskID))
}

// layout returns where this project lives, working it out on the first call
// and keeping the answer. New leaves it unresolved, so constructing a service
// needs no directory drudge can read.
func (service *DrudgerService) layout() (projectLayout, error) {
	if service.resolvedLayout != nil {
		return *service.resolvedLayout, nil
	}

	dir, err := common.WorkDir()
	if err != nil {
		return projectLayout{}, fmt.Errorf("could not work out which directory this project lives in: %w", err)
	}

	resolved := projectLayout{Dir: dir}
	service.resolvedLayout = &resolved
	return resolved, nil
}
