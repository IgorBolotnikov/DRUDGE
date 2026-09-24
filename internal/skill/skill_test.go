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
	skillPath := func(home string) string {
		return filepath.Join(home, ".claude", "skills", "DRUDGE", "SKILL.md")
	}

	tests := []struct {
		name            string
		harness         config.Harness
		existingContent string
		wantInstalled   bool
	}{
		{
			name:          "claude-code gets the skill",
			harness:       config.HarnessClaudeCode,
			wantInstalled: true,
		},
		{
			name:            "claude-code overwrites an existing skill",
			harness:         config.HarnessClaudeCode,
			existingContent: "an older skill",
			wantInstalled:   true,
		},
		{
			name:          "opencode gets nothing",
			harness:       config.HarnessOpencode,
			wantInstalled: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			if test.existingContent != "" {
				if err := os.MkdirAll(filepath.Dir(skillPath(home)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(skillPath(home), []byte(test.existingContent), 0o644); err != nil {
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

			written, readErr := os.ReadFile(skillPath(home))
			if !test.wantInstalled {
				if path != "" {
					t.Errorf("expected no path, got %q", path)
				}
				if !os.IsNotExist(readErr) {
					t.Errorf("expected no skill file, got error %v", readErr)
				}
				return
			}
			if path != skillPath(home) {
				t.Errorf("expected path %q, got %q", skillPath(home), path)
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
