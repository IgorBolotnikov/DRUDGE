package drudger

import (
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

// Env is the kind of sandbox a Drudger runs in.
type Env string

const (
	EnvDockerSbx Env = "docker-sbx"
)

// Harness is the coding agent a Drudger runs.
type Harness string

const (
	HarnessClaudeCode Harness = "claude-code"
	HarnessOpencode   Harness = "opencode"
)

// JSON keys of the config files, named in error messages so they match what a
// user writes there.
const (
	MaxConcurrentDrudgersKey = "maxConcurrentDrudgers"
	CreateTimeoutKey         = "sandboxTimeouts.createSeconds"
)

// Settings is what a DrudgerService reads from the configs.
type Settings struct {
	ProjectSlug string
	// Repositories are the repositories of the project, relative to the
	// project directory.
	Repositories []project.Repository
	Env          Env
	Harness      Harness
	// MaxConcurrentDrudgers is how many Drudgers may work on the project at
	// once.
	MaxConcurrentDrudgers int
	// PromptPath is the prompt file to hand an agent. Empty means the
	// built-in prompt.
	PromptPath      string
	SandboxTimeouts SandboxTimeouts
}

// SandboxTimeouts caps how long each sandbox command may run. A command that
// outruns its cap is killed.
type SandboxTimeouts struct {
	List   time.Duration
	Create time.Duration
	Remove time.Duration
}
