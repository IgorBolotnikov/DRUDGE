package cmd

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

var DrudgerCmd = &Cmd{
	Name: "drudger",
	Desc: "Drudger management commands",
	Subcommands: []*Cmd{
		{
			Name:  "list",
			Desc:  "List the Drudgers of the project",
			Help:  "List the Drudgers of the current project and what each one is doing.",
			Setup: func(*flag.FlagSet) func(args []string) error { return drudgerList },
		},
		{
			Name: "nuke",
			Args: []string{slotArg},
			Desc: "Delete a Drudger",
			Help: "Delete a Drudger's sandbox and drop it from the pool.\n" +
				"A Drudger with a running Session is refused unless you insist.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				isForced := fs.Bool(forceFlagName, false, "Nuke a working Drudger, killing its agent and fucking up its task")
				alias(fs, forceFlagShortName, forceFlagName)
				return func(args []string) error { return drudgerNuke(args[0], *isForced) }
			},
		},
		{
			Name: "reclaim",
			Desc: "Free the slots held by agents that are gone",
			Help: "Free the Drudger slots that are still claimed by an agent that is gone.\n\n" +
				"A Drudger frees its slot when its agent writes an exit file. An agent killed before that leaves the slot claimed, and a claimed slot counts against the concurrency limit.\n\n" +
				"This only rewrites the Drudgers file. No process is killed, no sandbox is removed and the tasks those slots held keep their status, so start one over with " + taskRerunCommand + ".",
			Setup: func(*flag.FlagSet) func(args []string) error { return drudgerReclaim },
		},
	},
}

const slotArg = "slot"

const (
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

func drudgerList([]string) error {
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
func drudgerReclaim([]string) error {
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
func drudgerNuke(slotText string, isForced bool) error {
	slot, err := parseDrudgerNukeArgs(slotText)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.drudger.NukeDrudger(deps.localCfg.ProjectSlug, slot, isForced)
}

func parseDrudgerNukeArgs(slot string) (int, error) {
	parsed, err := strconv.Atoi(slot)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%q is not a Drudger slot, slots are whole numbers starting at 1", slot)
	}
	return parsed, nil
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
