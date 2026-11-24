package copytaskid_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/copytaskid"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

func TestKind(t *testing.T) {
	cases := []struct {
		name          string
		taskID        task.TaskID
		wantVerdict   string
		wantClipboard tea.Msg
	}{
		{name: "copies the full ID and names the short one", taskID: testTaskID, wantVerdict: "3f9a1c2e copied to clipboard", wantClipboard: tea.SetClipboard(string(testTaskID))()},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{}, "uploader", []actions.Kind{copytaskid.Kind})

			command, cmd := commands.Start(views.CopyTaskIDMsg{TaskID: testCase.taskID})
			done, ok := cmd().(actions.DoneMsg)

			if !ok {
				t.Fatal("expected a done message")
			}
			wantCommand := actions.Command{Name: views.CopyTaskIDCommand, Target: "3f9a1c2e"}
			if command != wantCommand || done.Command != wantCommand {
				t.Errorf("expected command %v, got %v and %v", wantCommand, command, done.Command)
			}
			if done.Verdict != testCase.wantVerdict || done.IsSkipped || done.Err != nil {
				t.Errorf("expected verdict %q, got %q (skipped %v, error %v)", testCase.wantVerdict, done.Verdict, done.IsSkipped, done.Err)
			}
			if done.Cmd == nil || done.Cmd() != testCase.wantClipboard {
				t.Errorf("expected the clipboard write %v", testCase.wantClipboard)
			}
		})
	}
}
