// Package setup lays out and removes the drudge home directory.
package setup

// SetupResult is the outcome of one setup, with the paths in the order setup
// wrote them.
type SetupResult struct {
	DrudgeDir    string
	SchemaPaths  []string
	SkillPath    string // Empty when the harness takes no skill
	GlobalConfig ConfigFile
	ThemeConfig  ConfigFile
	// InterfaceConfigs holds the config files of the user interfaces, in the
	// order the service got them.
	InterfaceConfigs []ConfigFile
}

// SchemaFile is a bundled JSON schema setup writes to the schema directory
// under Name.
type SchemaFile struct {
	Name    string
	Content []byte
}

// InterfaceConfig is the config file of a user interface. Setup writes Schema
// to the schema directory under Name on every run. It writes Default to the
// drudge home directory under Name when that file is missing.
type InterfaceConfig struct {
	Name    string
	Schema  []byte
	Default any
}

// ConfigFile is a config file setup writes, with what setup did to it.
type ConfigFile struct {
	Path    string
	Outcome ConfigOutcome
}

// ConfigOutcome is what setup did to a config file.
type ConfigOutcome string

const (
	ConfigCreated   ConfigOutcome = "created"
	ConfigUpdated   ConfigOutcome = "updated"
	ConfigUnchanged ConfigOutcome = "unchanged"
)

// CleanupResult is the outcome of one cleanup.
type CleanupResult struct {
	DrudgeDir  string
	HasRemoved bool // False when there was no drudge home directory to remove
}
