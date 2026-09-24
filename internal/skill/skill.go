// Package skill holds the agent skills DRUDGE ships and installs them for a harness.
package skill

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
)

// Where Claude Code looks for a personal skill: a directory named after the
// skill, under the skills directory of its home directory.
const (
	claudeDirName   = ".claude"
	skillsDirName   = "skills"
	skillFileName   = "SKILL.md"
	drudgeSkillName = "DRUDGE"
)

//go:embed drudge/SKILL.md
var drudgeSkill []byte

// Drudge returns the bundled drudge skill.
func Drudge() []byte {
	return drudgeSkill
}

// InstallDrudge writes the drudge skill into home for harness and returns the
// path it wrote. An existing skill file is overwritten. A harness with no
// skill support gets nothing and a false.
func InstallDrudge(home string, harness config.Harness) (string, bool, error) {
	if harness != config.HarnessClaudeCode {
		return "", false, nil
	}

	skillDir := filepath.Join(home, claudeDirName, skillsDirName, drudgeSkillName)
	if err := common.EnsureDir(skillDir); err != nil {
		return "", false, err
	}
	path := filepath.Join(skillDir, skillFileName)
	if err := os.WriteFile(path, drudgeSkill, common.DefaultFilePerm); err != nil {
		return "", false, fmt.Errorf("could not write skill: %w", err)
	}
	return path, true, nil
}
