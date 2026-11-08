// Package style holds the presentation pieces every interface of drudge
// shares: the labels it shows and the theme roles they print in.
package style

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

// StatusRoles maps a task status to the theme role it prints in. A status left
// out, like todo, prints plain.
var StatusRoles = map[task.TaskStatus]string{
	task.StatusDraft:      theme.RoleMuted,
	task.StatusInProgress: theme.RoleInfo,
	task.StatusFuckedUp:   theme.RoleError,
	task.StatusUnmerged:   theme.RoleWarning,
	task.StatusDone:       theme.RoleSuccess,
}

// Labels for what drudge last saw of a Drudger. A Drudger is an agent in a
// sandbox over a workspace, so the health of a Drudger reports all three
// parts. A part that is fine stays quiet. A part drudge has not looked at yet
// says so plainly. A broken part is shouted, so it stands out in a listing of
// otherwise fine Drudgers.
const (
	HealthOkLabel        = "ok"
	HealthUncheckedLabel = "unchecked"

	SandboxGoneLabel      = "SANDBOX GONE"
	SandboxMisplacedLabel = "WRONG WORKSPACE"
	SandboxUncheckedLabel = "sandbox unchecked"

	WorkspaceGoneLabel      = "WORKSPACE GONE"
	WorkspaceMisplacedLabel = "WRONG WORKTREE"
	WorkspaceUncheckedLabel = "workspace unchecked"

	AgentRefusedLabel   = "AGENT REFUSED"
	AgentUncheckedLabel = "agent unchecked"

	// HealthPartSeparator joins the parts that have something to say.
	HealthPartSeparator = ", "
)

// HealthLabelRoles maps a health label to the theme role it prints in.
var HealthLabelRoles = map[string]string{
	HealthOkLabel: theme.RoleSuccess,

	HealthUncheckedLabel:    theme.RoleMuted,
	SandboxUncheckedLabel:   theme.RoleMuted,
	WorkspaceUncheckedLabel: theme.RoleMuted,
	AgentUncheckedLabel:     theme.RoleMuted,

	SandboxGoneLabel:        theme.RoleError,
	SandboxMisplacedLabel:   theme.RoleError,
	WorkspaceGoneLabel:      theme.RoleError,
	WorkspaceMisplacedLabel: theme.RoleError,
	AgentRefusedLabel:       theme.RoleError,
}
