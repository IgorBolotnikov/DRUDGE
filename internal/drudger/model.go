package drudger

import (
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// Drudger is an agent running in a reusable sandbox over a workspace of git
// worktrees. It outlives the Sessions that run in it, and at any moment it is
// either occupied by one Task or idle.
//
// The sandbox, the workspace and the agent break independently, so a Drudger
// has one state for each of them.
//
// A Drudger record is a snapshot of what drudge last observed, not a live
// view of the Drudger. LastChecked says how old that observation is.
type Drudger struct {
	Slot            int             // Pool position, starting at 1
	Sandbox         string          // Name of the real sandbox this Drudger works in
	Workspace       string          // Directory the agent works in, holding a worktree per repository
	TaskID          task.TaskID     // Task occupying the Drudger, empty when idle
	SandboxHealth   SandboxHealth   // What drudge last saw of the sandbox
	WorkspaceHealth WorkspaceHealth // What drudge last saw of the workspace
	AgentHealth     AgentHealth     // What drudge last saw of the agent
	LastChecked     time.Time       // When drudge last looked at this Drudger
}

// SandboxHealth is what drudge last saw of a Drudger's sandbox.
type SandboxHealth string

const (
	// SandboxUnchecked means drudge has not looked at the sandbox yet.
	SandboxUnchecked SandboxHealth = ""
	// SandboxUsable means the sandbox is there and holds the project workspace.
	SandboxUsable SandboxHealth = "usable"
	// SandboxGone means the sandbox is not there.
	SandboxGone SandboxHealth = "gone"
	// SandboxMisplaced means the sandbox is there but is mounted on another
	// workspace.
	SandboxMisplaced SandboxHealth = "misplaced"
)

// WorkspaceHealth is what drudge last saw of the workspace a Drudger works in.
// It is checked at every handover.
type WorkspaceHealth string

const (
	// WorkspaceUnchecked means drudge has not looked at the workspace yet.
	WorkspaceUnchecked WorkspaceHealth = ""
	// WorkspaceUsable means every repository of the project is checked out
	// where the Drudger expects it.
	WorkspaceUsable WorkspaceHealth = "usable"
	// WorkspaceGone means a worktree a repository still knows has no directory
	// left at its path.
	WorkspaceGone WorkspaceHealth = "gone"
	// WorkspaceMisplaced means a worktree path holds a directory the
	// repository does not know as a worktree.
	WorkspaceMisplaced WorkspaceHealth = "misplaced"
)

// AgentHealth is what drudge last saw of the agent inside a Drudger's sandbox.
// It answers whether the agent can work at all.
type AgentHealth string

const (
	// AgentUnchecked means drudge has not seen this agent finish a Session yet.
	AgentUnchecked AgentHealth = ""
	// AgentReady means the agent got through to the vendor and did its work.
	AgentReady AgentHealth = "ready"
	// AgentRefused means the vendor turned the agent away, so it can do
	// nothing until whatever the vendor named is fixed.
	AgentRefused AgentHealth = "refused"
)

// Idle reports whether no Task occupies the Drudger.
func (drudger *Drudger) Idle() bool {
	return drudger.TaskID == ""
}

// HealthPart names one of the parts of a Drudger that break independently.
type HealthPart string

const (
	SandboxPart   HealthPart = "sandbox"
	WorkspacePart HealthPart = "workspace"
	AgentPart     HealthPart = "agent"
)

// HealthFault is a part of a Drudger that is not fine. State holds the health
// value of that part as it is stored.
type HealthFault struct {
	Part  HealthPart
	State string
}

// HealthSummary is what drudge last saw of a Drudger as a whole.
type HealthSummary struct {
	IsUnchecked bool          // Drudge has not looked at any part
	Faults      []HealthFault // Parts that are not fine, in the order sandbox, workspace, agent
}

// IsOk reports whether every part of the Drudger is fine.
func (summary HealthSummary) IsOk() bool {
	return !summary.IsUnchecked && len(summary.Faults) == 0
}

// Health summarizes what drudge last saw of the sandbox, the workspace and the
// agent. Three unchecked parts read as one unchecked Drudger and carry no
// faults. Otherwise every part that is not usable or ready is a fault, and so
// is a state this build does not know.
func (drudger *Drudger) Health() HealthSummary {
	if drudger.SandboxHealth == SandboxUnchecked && drudger.WorkspaceHealth == WorkspaceUnchecked && drudger.AgentHealth == AgentUnchecked {
		return HealthSummary{IsUnchecked: true}
	}

	var faults []HealthFault
	if drudger.SandboxHealth != SandboxUsable {
		faults = append(faults, HealthFault{Part: SandboxPart, State: string(drudger.SandboxHealth)})
	}
	if drudger.WorkspaceHealth != WorkspaceUsable {
		faults = append(faults, HealthFault{Part: WorkspacePart, State: string(drudger.WorkspaceHealth)})
	}
	if drudger.AgentHealth != AgentReady {
		faults = append(faults, HealthFault{Part: AgentPart, State: string(drudger.AgentHealth)})
	}
	return HealthSummary{Faults: faults}
}
