package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"drudge/internal/drudger"
	"drudge/internal/task"
)

var DrudgerCmd = &Cmd{
	Name:  "drudger",
	Usage: "drudger <subcommand>",
	Desc:  "Drudger management commands",
	Run:   runDrudger,
}

const (
	drudgerUsage     = "usage: drg drudger <subcommand>"
	drudgerListUsage = "usage: drg drudger list"

	idleLabel = "idle"

	neverCheckedLabel = "never"
)

func runDrudger(args []string) error {
	if len(args) < 1 {
		return errors.New(drudgerUsage)
	}

	switch args[0] {
	case "list":
		return drudgerList(args[1:])
	default:
		return fmt.Errorf("unknown drudger subcommand: %s", args[0])
	}
}

func drudgerList(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(drudgerListUsage)
		fmt.Println()
		fmt.Println("List the Drudgers of the current project and what each one is doing.")
		return nil
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	drudgers, err := deps.drudger.ListDrudgers(deps.localCfg.ProjectSlug)
	if err != nil {
		return err
	}

	if len(drudgers) == 0 {
		deps.log.Info("Project %s has no Drudgers, the first one is built when you run a task", deps.localCfg.ProjectSlug)
		return nil
	}

	columns := []column{
		{Title: "SLOT", Width: 4},
		{Title: "SANDBOX", Width: 40},
		{Title: "TASK", Width: task.ShortIDLength},
		{Title: "LAST CHECKED"},
	}
	now := time.Now().UTC()
	rows := make([][]string, 0, len(drudgers))
	for _, entry := range drudgers {
		rows = append(rows, []string{
			strconv.Itoa(entry.Slot),
			entry.Sandbox,
			occupyingTask(entry),
			formatLastChecked(entry.LastChecked, now),
		})
	}

	printList(deps.log, "Drudgers", columns, rows)
	return nil
}

func occupyingTask(entry *drudger.Drudger) string {
	if entry.Idle() {
		return idleLabel
	}
	return shortTaskID(entry.TaskID)
}

func formatLastChecked(lastChecked time.Time, now time.Time) string {
	if lastChecked.IsZero() {
		return neverCheckedLabel
	}

	elapsed := now.Sub(lastChecked)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
}
