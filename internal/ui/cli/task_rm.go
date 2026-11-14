package cli

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/cli/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/report"
)

// taskRemove deletes one task and the run directory of its Sessions.
func taskRemove(taskID task.TaskID, isForced bool) error {
	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	confirm := func(removal task.Removal) (bool, error) { return confirmTaskRemoval(deps.out, removal) }
	return deps.tasks.RemoveTask(deps.localCfg.ProjectSlug, taskID, isForced, deps.drudger, confirm)
}

// confirmTaskRemoval asks the user whether a task should go.
func confirmTaskRemoval(out *printer.Printer, removal task.Removal) (bool, error) {
	return ConfirmDeletion(out, describeRemoval(removal))
}

func describeRemoval(removal task.Removal) string {
	return report.Removal(removal)
}
