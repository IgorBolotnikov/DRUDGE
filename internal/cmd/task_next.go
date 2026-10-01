package cmd

import (
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
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
	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	pick, err := deps.tasks.NextTask(deps.localCfg.ProjectSlug)
	if err != nil {
		return err
	}

	printNext(deps.out, pick)
	return nil
}

// printNext prints the picked task. With no task picked, it prints the blocked
// todo tasks, or that there are none.
func printNext(out *printer.Printer, pick task.Pick) {
	var lines []string
	switch {
	case pick.Task != nil:
		lines = append(lines, fmt.Sprintf(nextTaskLine, task.ShortID(pick.Task.ID), pick.Task.Title))
	case len(pick.Blocked) == 0:
		lines = append(lines, "No task can be started. There are no todo tasks.")
	default:
		lines = append(lines, blockedTaskLines(pick.Blocked)...)
	}

	out.View(lines)
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
