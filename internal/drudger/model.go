package drudger

import (
	"time"

	"drudge/internal/task"
)

// Drudger is a reusable sandbox that does the work. It outlives the Sessions
// that run in it, and at any moment it is either occupied by one Task or idle.
//
// A Drudger record is a snapshot of what drudge last observed, never a live
// view of the sandbox. LastChecked says how old that observation is.
type Drudger struct {
	Slot        int         // Pool position, starting at 1
	Sandbox     string      // Name of the real sandbox this Drudger works in
	TaskID      task.TaskID // Task occupying the Drudger, empty when idle
	LastChecked time.Time   // When drudge last looked at this Drudger
}

// Idle reports whether no Task occupies the Drudger.
func (drudger *Drudger) Idle() bool {
	return drudger.TaskID == ""
}
