package cmd

import (
	"fmt"
	"strings"

	"drudge/internal/common"
	"drudge/internal/drudger"
	"drudge/internal/task"
)

// nextTaskLine lays out a task drg task next names.
const nextTaskLine = "%s  %s"

// blockedTaskLine lays out a blocked todo task, the title padded to the
// widest of them.
const blockedTaskLine = "%s%s  %-*s  blocked by %s"

// holdingBlockerSeparator joins the short ids of the blockers holding a task.
const holdingBlockerSeparator = ", "

// taskNext prints the task that can be started now.
func taskNext(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskNextUsage)
		fmt.Println()
		fmt.Println("Print the oldest todo task whose blockers are all done. It starts nothing.")
		fmt.Println("Work of a blocker that is not merged yet is listed under the task.")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q, %s", args[0], taskNextUsage)
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	pick, err := deps.tasks.NextTask(deps.localCfg.ProjectSlug)
	if err != nil {
		return err
	}

	unmerged, err := deps.drudger.UnmergedWork(pick.Blockers)
	if err != nil {
		return err
	}

	printNext(deps.log, pick, unmerged)
	return nil
}

// printNext prints the picked task and the unmerged work of its blockers. With
// no task picked, it prints the blocked todo tasks, or that there are none.
func printNext(log *common.Logger, pick task.Pick, unmerged map[task.TaskID][]drudger.UnmergedWork) {
	var lines []string
	switch {
	case pick.Task != nil:
		lines = append(lines, fmt.Sprintf(nextTaskLine, task.ShortID(pick.Task.ID), pick.Task.Title))
		lines = append(lines, waitingMergeLines(pick.Blockers, unmerged)...)
	case len(pick.Blocked) == 0:
		lines = append(lines, "No task can be started. There are no todo tasks.")
	default:
		lines = append(lines, blockedTaskLines(pick.Blocked)...)
	}

	for _, line := range lines {
		// A task title may hold a percent sign.
		log.Info("%s", line)
	}
}

// waitingMergeLines lists each blocker with unmerged work and that work under
// it. Blockers with all of their work merged get no lines.
func waitingMergeLines(blockers []task.Blocker, unmerged map[task.TaskID][]drudger.UnmergedWork) []string {
	var lines []string
	for _, blocker := range blockers {
		work := unmerged[blocker.ID]
		if len(work) == 0 {
			continue
		}
		lines = append(lines, listIndent+fmt.Sprintf(nextTaskLine, task.ShortID(blocker.ID), blocker.Task.Title))
		for _, line := range drudger.FormatUnmergedWork(work) {
			lines = append(lines, listIndent+listIndent+line)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return append([]string{"", "Waiting on a merge:"}, lines...)
}

func blockedTaskLines(blocked []task.BlockedTask) []string {
	heading := fmt.Sprintf("No task can be started. %d todo tasks are all blocked:", len(blocked))
	if len(blocked) == 1 {
		heading = "No task can be started. The only todo task is blocked:"
	}

	width := 0
	for _, entry := range blocked {
		width = max(width, len(entry.Task.Title))
	}

	lines := []string{heading}
	for _, entry := range blocked {
		holding := make([]string, 0, len(entry.Holding))
		for _, id := range entry.Holding {
			holding = append(holding, task.ShortID(id))
		}
		lines = append(lines, fmt.Sprintf(
			blockedTaskLine,
			listIndent, task.ShortID(entry.Task.ID), width, entry.Task.Title, strings.Join(holding, holdingBlockerSeparator),
		))
	}
	return lines
}
