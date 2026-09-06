package drudger

import (
	"time"

	"drudge/internal/task"
)

// Drudger is a reusable sandbox that works on Tasks. It outlives the Sessions
// that run in it, and at any moment it is either occupied by one Task or idle.
//
// A Drudger record is a snapshot of what drudge last observed, not a live
// view of the sandbox. LastChecked says how old that observation is.
type Drudger struct {
	Slot        int         // Pool position, starting at 1
	Sandbox     string      // Name of the real sandbox this Drudger works in
	TaskID      task.TaskID // Task occupying the Drudger, empty when idle
	Health      Health      // What drudge last saw of the sandbox
	LastChecked time.Time   // When drudge last looked at this Drudger
}

// Health is what drudge last saw of a Drudger's sandbox. It describes the
// sandbox and nothing else. How the agent's work is going is a Session
// question, answered from the run directory.
type Health string

const (
	// HealthUnknown means drudge has not looked at the sandbox yet.
	HealthUnknown Health = ""
	// HealthUsable means the sandbox is there and holds the project workspace.
	HealthUsable Health = "usable"
	// HealthGone means the sandbox is not there.
	HealthGone Health = "gone"
	// HealthMisplaced means the sandbox is there but is mounted on another
	// workspace, so an agent in it would edit the wrong repository.
	HealthMisplaced Health = "misplaced"
)

// Idle reports whether no Task occupies the Drudger.
func (drudger *Drudger) Idle() bool {
	return drudger.TaskID == ""
}
