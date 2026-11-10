// Package tuitest holds what the tests of the TUI packages share: golden
// files, styles and task fixtures.
package tuitest

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
)

var shouldUpdateGoldens = flag.Bool("update", false, "Rewrite the golden files of the TUI from what it renders")

const goldenDir = "testdata"

// updateCommand regenerates the goldens of every TUI package. The packages
// without goldens do not define the update flag, so a pattern over the whole
// TUI fails.
const updateCommand = "go test ./internal/ui/tui ./internal/ui/tui/views/... -update"

// Key presses that move a cursor.
var (
	KeyDown      = tea.KeyPressMsg{Code: 'j', Text: "j"}
	KeyUp        = tea.KeyPressMsg{Code: 'k', Text: "k"}
	KeyArrowDown = tea.KeyPressMsg{Code: tea.KeyDown}
	KeyTop       = tea.KeyPressMsg{Code: 'g', Text: "g"}
	KeyBottom    = tea.KeyPressMsg{Code: 'g', Text: "G", Mod: tea.ModShift}
)

// Styles builds the styles of a theme with hex colors. Goldens strip the
// colors.
func Styles() *styles.Styles {
	return styles.New(theme.NewTheme("nord"))
}

// AssertGolden strips the colors from a screen and compares it with a golden
// file in the testdata directory of the package under test.
func AssertGolden(t *testing.T, name string, screen string) {
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
		t.Fatalf("%v, regenerate it with %s", err, updateCommand)
	}
	if got := plain.String(); got != string(want) {
		t.Errorf("screen differs from %s, regenerate it with %s if the change is intended\ngot:\n%s\nwant:\n%s", goldenPath, updateCommand, got, want)
	}
}

// ListedTasks returns a parent with a child, a blocked task, a task with a
// long title and a draft.
func ListedTasks() []task.ListedTask {
	parent := &task.Task{ID: "3f9a1c2e-0000-0000-0000-000000000001", Title: "Add retry to uploader", Status: task.StatusInProgress, TicketID: "UP-12"}
	return []task.ListedTask{
		{Task: parent},
		{Task: &task.Task{ID: "5b20d7aa-0000-0000-0000-000000000002", Title: "Write the backoff", Status: task.StatusDone, ParentTaskID: parent.ID}, IsUnderParent: true},
		{Task: &task.Task{ID: "7c1e4f90-0000-0000-0000-000000000003", Title: "Wire retries into the CLI", Status: task.StatusTodo}, Holding: []task.TaskID{parent.ID}},
		{Task: &task.Task{ID: "9d8b2a11-0000-0000-0000-000000000004", Title: "Fix the flaky upload test that fails on slow machines", Status: task.StatusFuckedUp, TicketID: "UP-7"}},
		{Task: &task.Task{ID: "a01f3b55-0000-0000-0000-000000000005", Title: "Draft the release notes", Status: task.StatusDraft}},
	}
}
