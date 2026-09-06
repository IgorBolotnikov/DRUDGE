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

// Labels for what drudge last saw of a Drudger's sandbox. A usable sandbox is
// the normal case and stays quiet. A broken one is shouted so it stands out in
// a listing of otherwise fine Drudgers.
const (
	healthUsableLabel    = "ok"
	healthGoneLabel      = "SANDBOX GONE"
	healthMisplacedLabel = "WRONG WORKSPACE"
	healthUnknownLabel   = "unchecked"

	// healthColumnWidth fits the longest label.
	healthColumnWidth = len(healthMisplacedLabel)
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
		{Title: "HEALTH", Width: healthColumnWidth},
		{Title: "LAST CHECKED"},
	}
	now := time.Now().UTC()
	rows := make([][]string, 0, len(drudgers))
	for _, entry := range drudgers {
		rows = append(rows, []string{
			strconv.Itoa(entry.Slot),
			entry.Sandbox,
			occupyingTask(entry),
			formatHealth(entry.Health),
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

func formatHealth(health drudger.Health) string {
	switch health {
	case drudger.HealthUsable:
		return healthUsableLabel
	case drudger.HealthGone:
		return healthGoneLabel
	case drudger.HealthMisplaced:
		return healthMisplacedLabel
	case drudger.HealthUnknown:
		return healthUnknownLabel
	default:
		return string(health)
	}
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
