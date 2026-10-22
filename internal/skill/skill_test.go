package skill

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/config"
)

func TestDrudge_StartsWithFrontMatterNamingDrudge(t *testing.T) {
	content := string(Drudge())

	frontMatter, _, isFound := strings.Cut(strings.TrimPrefix(content, "---\n"), "\n---\n")
	if !strings.HasPrefix(content, "---\n") || !isFound {
		t.Fatalf("expected the skill to start with a front matter block, got %q", content)
	}
	if !strings.Contains(frontMatter, "name: DRUDGE\n") {
		t.Errorf("expected the front matter to name DRUDGE, got %q", frontMatter)
	}
	if !strings.Contains(frontMatter, "description: ") {
		t.Errorf("expected the front matter to carry a description, got %q", frontMatter)
	}
}

func TestInstallDrudge(t *testing.T) {
	claudeCodeSkillPath := filepath.Join(".claude", "skills", "DRUDGE", "SKILL.md")
	opencodeSkillPath := filepath.Join(".config", "opencode", "skills", "DRUDGE", "SKILL.md")

	tests := []struct {
		name            string
		harness         config.Harness
		skillPath       string
		existingContent string
		wantInstalled   bool
	}{
		{
			name:          "claude-code gets the skill",
			harness:       config.HarnessClaudeCode,
			skillPath:     claudeCodeSkillPath,
			wantInstalled: true,
		},
		{
			name:            "claude-code overwrites an existing skill",
			harness:         config.HarnessClaudeCode,
			skillPath:       claudeCodeSkillPath,
			existingContent: "an older skill",
			wantInstalled:   true,
		},
		{
			name:          "opencode gets the skill",
			harness:       config.HarnessOpencode,
			skillPath:     opencodeSkillPath,
			wantInstalled: true,
		},
		{
			name:            "opencode overwrites an existing skill",
			harness:         config.HarnessOpencode,
			skillPath:       opencodeSkillPath,
			existingContent: "an older skill",
			wantInstalled:   true,
		},
		{
			name:          "an unknown harness gets nothing",
			harness:       config.Harness("vim"),
			skillPath:     claudeCodeSkillPath,
			wantInstalled: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			skillPath := filepath.Join(home, test.skillPath)
			if test.existingContent != "" {
				if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(skillPath, []byte(test.existingContent), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			path, didInstall, err := InstallDrudge(home, test.harness)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if didInstall != test.wantInstalled {
				t.Fatalf("expected installed %v, got %v", test.wantInstalled, didInstall)
			}

			written, readErr := os.ReadFile(skillPath)
			if !test.wantInstalled {
				if path != "" {
					t.Errorf("expected no path, got %q", path)
				}
				if !os.IsNotExist(readErr) {
					t.Errorf("expected no skill file, got error %v", readErr)
				}
				return
			}
			if path != skillPath {
				t.Errorf("expected path %q, got %q", skillPath, path)
			}
			if readErr != nil {
				t.Fatalf("could not read the skill: %v", readErr)
			}
			if !bytes.Equal(written, Drudge()) {
				t.Errorf("expected the embedded skill, got %q", written)
			}
		})
	}
}
