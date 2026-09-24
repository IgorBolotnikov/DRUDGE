package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"drudge/internal/common"
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
	drudgerUsage        = "usage: drg drudger <subcommand>"
	drudgerListUsage    = "usage: drg drudger list"
	drudgerNukeUsage    = "usage: drg drudger nuke <slot> [" + forceFlagShort + "]"
	drudgerReclaimUsage = "usage: drg drudger reclaim"

	idleLabel = "idle"

	// neverLabel stands for a moment that never happened.
	neverLabel = "never"
)

// Labels for what drudge last saw of a Drudger. A Drudger is an agent in a
// sandbox over a workspace, so the health column reports all three parts. A
// part that is fine stays quiet. A part drudge has not looked at yet says so
// plainly. A broken part is shouted, so it stands out in a listing of
// otherwise fine Drudgers.
const (
	healthOkLabel        = "ok"
	healthUncheckedLabel = "unchecked"

	sandboxGoneLabel      = "SANDBOX GONE"
	sandboxMisplacedLabel = "WRONG WORKSPACE"
	sandboxUncheckedLabel = "sandbox unchecked"

	workspaceGoneLabel      = "WORKSPACE GONE"
	workspaceMisplacedLabel = "WRONG WORKTREE"
	workspaceUncheckedLabel = "workspace unchecked"

	agentRefusedLabel   = "AGENT REFUSED"
	agentUncheckedLabel = "agent unchecked"

	// healthPartSeparator joins the parts that have something to say.
	healthPartSeparator = ", "

	// healthColumnWidth fits the widest row a Drudger can show: two parts
	// drudge has not looked at and one that is broken. Three unchecked parts
	// read as one label.
	healthColumnWidth = len(sandboxUncheckedLabel) + len(healthPartSeparator) + len(workspaceUncheckedLabel) + len(healthPartSeparator) + len(agentRefusedLabel)
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
	case "reclaim":
		return drudgerReclaim(args[1:])
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

	printDrudgers(deps.log, deps.localCfg.ProjectSlug, drudgers, time.Now().UTC())
	return nil
}

// printDrudgers lists the Drudgers of a project in the order given, one row
// each.
func printDrudgers(log *common.Logger, projectSlug string, drudgers []*drudger.Drudger, now time.Time) {
	if len(drudgers) == 0 {
		log.Info("Project %s has no Drudgers, the first one is built when you run a task", projectSlug)
		return
	}

	columns := []column{
		{Title: "SLOT", Width: 4},
		{Title: "DRUDGER", Width: 40},
		{Title: "TASK", Width: task.ShortIDLength},
		{Title: "HEALTH", Width: healthColumnWidth},
		{Title: "LAST CHECKED"},
	}
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

	printList(log, "Drudgers", columns, rows)
}

// drudgerReclaim frees the Drudger slots whose agent is gone.
func drudgerReclaim(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(drudgerReclaimUsage)
		fmt.Println()
		fmt.Println("Free the Drudger slots that are still claimed by an agent that is gone.")
		fmt.Println()
		fmt.Println("A Drudger frees its slot when its agent writes an exit file. An agent killed")
		fmt.Println("before that leaves the slot claimed, and a claimed slot counts against the")
		fmt.Println("concurrency limit.")
		fmt.Println()
		fmt.Println("This only rewrites the Drudgers file. No process is killed, no sandbox is")
		fmt.Println("removed and the tasks those slots held keep their status, so start one over")
		fmt.Printf("with %s.\n", taskRerunCommand)
		return nil
	}

	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q, %s", args[0], drudgerReclaimUsage)
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	freed, err := deps.drudger.ReclaimDrudgers(deps.localCfg.ProjectSlug)
	if err != nil {
		return err
	}

	if len(freed) == 0 {
		deps.log.Info("Every Drudger of project %s is either idle or working, nothing to reclaim", deps.localCfg.ProjectSlug)
		return nil
	}

	for _, entry := range freed {
		deps.log.Info("Drudger %d (%s) was holding task %s with no agent in it, %s", entry.Slot, entry.Sandbox, task.ShortID(entry.TaskID), entry.Reason)
	}
	deps.log.Info("Those slots are free. Start a task over with %s <task-id>.", taskRerunCommand)
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

	slot, isForced, err := parseDrudgerNukeArgs(args)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.drudger.NukeDrudger(deps.localCfg.ProjectSlug, slot, isForced)
}

func parseDrudgerNukeArgs(args []string) (int, bool, error) {
	var slot string
	isForced := false

	for _, arg := range args {
		switch {
		case arg == forceFlag || arg == forceFlagShort:
			isForced = true
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
	return parsed, isForced, nil
}

func occupyingTask(entry *drudger.Drudger) string {
	if entry.Idle() {
		return idleLabel
	}
	return task.ShortID(entry.TaskID)
}

// formatHealth renders what drudge last saw of a Drudger. Only a Drudger whose
// sandbox, workspace and agent are all fine reads as ok. Anything else names
// the part that is at fault, so the reader knows which one to fix.
//
// TODO: move this to the drudger package to be reused in other UI layers.
func formatHealth(entry *drudger.Drudger) string {
	// The three parts of a Drudger drudge has not looked at yet share one
	// label.
	if entry.SandboxHealth == drudger.SandboxUnchecked && entry.WorkspaceHealth == drudger.WorkspaceUnchecked && entry.AgentHealth == drudger.AgentUnchecked {
		return healthUncheckedLabel
	}

	parts := make([]string, 0, 3)
	if label := sandboxHealthLabel(entry.SandboxHealth); label != "" {
		parts = append(parts, label)
	}
	if label := workspaceHealthLabel(entry.WorkspaceHealth); label != "" {
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

// workspaceHealthLabel names a workspace that is not fine. A usable workspace
// has nothing to report and gets an empty label.
func workspaceHealthLabel(health drudger.WorkspaceHealth) string {
	switch health {
	case drudger.WorkspaceUsable:
		return ""
	case drudger.WorkspaceGone:
		return workspaceGoneLabel
	case drudger.WorkspaceMisplaced:
		return workspaceMisplacedLabel
	case drudger.WorkspaceUnchecked:
		return workspaceUncheckedLabel
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
