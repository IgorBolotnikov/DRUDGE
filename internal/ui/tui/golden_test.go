package tui

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

var shouldUpdateGoldens = flag.Bool("update", false, "Rewrite the golden files of the TUI from what it renders")

const goldenDir = "testdata"

// testStyles builds the styles of a theme with hex colors, which every golden
// renders without.
func testStyles() *Styles {
	return NewStyles(theme.NewTheme("nord"))
}

// assertGolden compares screen, rendered with the no-color profile, with the
// golden file name.
func assertGolden(t *testing.T, name string, screen string) {
	t.Helper()
	var plain bytes.Buffer
	writer := &colorprofile.Writer{Forward: &plain, Profile: colorprofile.NoTTY}
	if _, err := writer.Write([]byte(screen)); err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join(goldenDir, name+".golden")
	if *shouldUpdateGoldens {
		if err := os.WriteFile(goldenPath, plain.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v, regenerate it with go test ./internal/ui/tui -update", err)
	}
	if got := plain.String(); got != string(want) {
		t.Errorf("screen differs from %s\ngot:\n%s\nwant:\n%s", goldenPath, got, want)
	}
}
