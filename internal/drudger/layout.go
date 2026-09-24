package drudger

import (
	"fmt"
	"path/filepath"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// slotDirPrefix names the workspace directory of a Drudger slot, and the slot
// number follows it.
const slotDirPrefix = "slot-"

// projectLayout is where a project lives on disk, where the files of its runs
// go and where its Drudgers work.
type projectLayout struct {
	// Dir is the directory drudge was invoked in.
	Dir string
}

// RunDir returns the directory the run files of a task live in.
func (layout projectLayout) RunDir(taskID task.TaskID) string {
	return common.RunDir(layout.Dir, string(taskID))
}

// RunsDir returns the directory holding the run files of every task. A
// Drudger's sandbox mounts it, so the agent writes its stream where the host
// reads it.
func (layout projectLayout) RunsDir() string {
	return common.RunsDir(layout.Dir)
}

// WorkspaceRoot returns the directory the Drudger of a slot works in. The
// worktree of each repository sits under it, and it is the agent's working
// directory.
func (layout projectLayout) WorkspaceRoot(slot int) string {
	return filepath.Join(common.WorktreesDir(layout.Dir), fmt.Sprintf("%s%d", slotDirPrefix, slot))
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
