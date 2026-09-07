package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	drudgerNukeUsage = "usage: drg drudger nuke <slot> [" + forceFlagShort + "]"

	idleLabel = "idle"

	// neverLabel stands for a moment that never happened.
	neverLabel = "never"
)

// Labels for what drudge last saw of a Drudger. A Drudger is an agent in a
// sandbox, so the health column reports both parts. A part that is fine stays
// quiet. A part drudge has not looked at yet says so plainly. A broken part is
// shouted, so it stands out in a listing of otherwise fine Drudgers.
const (
	healthOkLabel        = "ok"
	healthUncheckedLabel = "unchecked"

	sandboxGoneLabel      = "SANDBOX GONE"
	sandboxMisplacedLabel = "WRONG WORKSPACE"
	sandboxUncheckedLabel = "sandbox unchecked"

	agentRefusedLabel   = "AGENT REFUSED"
	agentUncheckedLabel = "agent unchecked"

	// healthPartSeparator joins the two parts when both have something to say.
	healthPartSeparator = ", "

	// healthColumnWidth fits the longest pair of labels.
	healthColumnWidth = len(sandboxMisplacedLabel) + len(healthPartSeparator) + len(agentRefusedLabel)
)

func runDrudger(args []string) error {
	if len(args) < 1 {
		return errors.New(drudgerUsage)
	}

	switch args[0] {
	case "list":
		return drudgerList(args[1:])
	case "nuke":
		return drudgerNuke(args[1:])
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
			formatHealth(entry),
			formatAgo(entry.LastChecked, now),
		})
	}

	printList(deps.log, "Drudgers", columns, rows)
	return nil
}

// drudgerNuke destroys one Drudger and fucks up the task worked on, if any.
func drudgerNuke(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(drudgerNukeUsage)
		fmt.Println()
		fmt.Println("Delete a Drudger's sandbox and drop it from the pool.")
		fmt.Println("A Drudger with a running Session is refused unless you insist.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf("  %s, %s  Nuke a working Drudger, killing its agent and fucking up its task\n", forceFlagShort, forceFlag)
		return nil
	}

	slot, force, err := parseDrudgerNukeArgs(args)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.drudger.NukeDrudger(deps.localCfg.ProjectSlug, slot, force)
}

func parseDrudgerNukeArgs(args []string) (int, bool, error) {
	var slot string
	force := false

	for _, arg := range args {
		switch {
		case arg == forceFlag || arg == forceFlagShort:
			force = true
		case strings.HasPrefix(arg, "-"):
			return 0, false, fmt.Errorf("unknown flag %q, %s", arg, drudgerNukeUsage)
		case slot == "":
			slot = arg
		default:
			return 0, false, fmt.Errorf("unexpected argument %q, drg drudger nuke takes a single slot", arg)
		}
	}

	if slot == "" {
		return 0, false, fmt.Errorf("slot is required, %s", drudgerNukeUsage)
	}

	parsed, err := strconv.Atoi(slot)
	if err != nil || parsed < 1 {
		return 0, false, fmt.Errorf("%q is not a Drudger slot, slots are whole numbers starting at 1", slot)
	}
	return parsed, force, nil
}

func occupyingTask(entry *drudger.Drudger) string {
	if entry.Idle() {
		return idleLabel
	}
	return shortTaskID(entry.TaskID)
}

// formatHealth renders what drudge last saw of a Drudger. Only a Drudger whose
// sandbox and agent are both fine reads as ok. Anything else names the part
// that is at fault, so the reader knows which one to fix.
func formatHealth(entry *drudger.Drudger) string {
	// Both parts of a Drudger drudge has not looked at yet share one label.
	if entry.SandboxHealth == drudger.SandboxUnchecked && entry.AgentHealth == drudger.AgentUnchecked {
		return healthUncheckedLabel
	}

	parts := make([]string, 0, 2)
	if label := sandboxHealthLabel(entry.SandboxHealth); label != "" {
		parts = append(parts, label)
	}
	if label := agentHealthLabel(entry.AgentHealth); label != "" {
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return healthOkLabel
	}
	return strings.Join(parts, healthPartSeparator)
}

// sandboxHealthLabel names a sandbox that is not fine. A usable sandbox has
// nothing to report and gets an empty label.
func sandboxHealthLabel(health drudger.SandboxHealth) string {
	switch health {
	case drudger.SandboxUsable:
		return ""
	case drudger.SandboxGone:
		return sandboxGoneLabel
	case drudger.SandboxMisplaced:
		return sandboxMisplacedLabel
	case drudger.SandboxUnchecked:
		return sandboxUncheckedLabel
	default:
		return string(health)
	}
}

// agentHealthLabel names an agent that is not fine. A ready agent has nothing
// to report and gets an empty label.
func agentHealthLabel(health drudger.AgentHealth) string {
	switch health {
	case drudger.AgentReady:
		return ""
	case drudger.AgentRefused:
		return agentRefusedLabel
	case drudger.AgentUnchecked:
		return agentUncheckedLabel
	default:
		return string(health)
	}
}

// formatAgo renders roughly how long ago a moment was.
func formatAgo(moment time.Time, now time.Time) string {
	if moment.IsZero() {
		return neverLabel
	}

	elapsed := now.Sub(moment)
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
