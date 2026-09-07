package drudger

import (
	"time"

	"drudge/internal/task"
)

// Drudger is an agent running in a reusable sandbox. It outlives the Sessions
// that run in it, and at any moment it is either occupied by one Task or idle.
//
// The agent and the sandbox break independently, so a Drudger has one state
// for each of them. A sandbox that is perfectly fine can hold an agent the
// vendor turned away.
//
// A Drudger record is a snapshot of what drudge last observed, not a live
// view of the Drudger. LastChecked says how old that observation is.
type Drudger struct {
	Slot          int           // Pool position, starting at 1
	Sandbox       string        // Name of the real sandbox this Drudger works in
	TaskID        task.TaskID   // Task occupying the Drudger, empty when idle
	SandboxHealth SandboxHealth // What drudge last saw of the sandbox
	AgentHealth   AgentHealth   // What drudge last saw of the agent
	LastChecked   time.Time     // When drudge last looked at this Drudger
}

// SandboxHealth is what drudge last saw of a Drudger's sandbox. It is learned
// from the sandbox listing every time a Drudger is put to work.
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

// AgentHealth is what drudge last saw of the agent inside a Drudger's sandbox.
// It answers whether the agent can work at all, which is a different question
// from how a Session is going. It is learned from the Session a Drudger
// finished.
type AgentHealth string

const (
	// AgentUnchecked means drudge has not seen this agent finish a Session yet.
	AgentUnchecked AgentHealth = ""
	// AgentReady means the agent got through to the vendor and did its work.
	// Whether that work came to anything is a Session question.
	AgentReady AgentHealth = "ready"
	// AgentRefused means the vendor turned the agent away, so it can do
	// nothing until whatever the vendor named is fixed.
	AgentRefused AgentHealth = "refused"
)

// Idle reports whether no Task occupies the Drudger.
func (drudger *Drudger) Idle() bool {
	return drudger.TaskID == ""
}
