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
}

// ConfigFile is a config file setup writes only when it is missing.
type ConfigFile struct {
	Path       string
	HasExisted bool // Setup left the file as it was
}

// CleanupResult is the outcome of one cleanup.
type CleanupResult struct {
	DrudgeDir  string
	HasRemoved bool // False when there was no drudge home directory to remove
}
