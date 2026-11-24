package copytaskid_test

import (
	"reflect"
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
			steps := sequenceSteps(t, cmd())

			if len(steps) != 2 {
				t.Fatalf("expected the clipboard write and the done message, got %d steps", len(steps))
			}
			if steps[0] != testCase.wantClipboard {
				t.Errorf("expected the clipboard write %v first, got %v", testCase.wantClipboard, steps[0])
			}
			done, ok := steps[1].(actions.DoneMsg)
			if !ok {
				t.Fatalf("expected a done message second, got %T", steps[1])
			}
			wantCommand := actions.Command{Name: views.CopyTaskIDCommand, Target: "3f9a1c2e"}
			if command != wantCommand || done.Command != wantCommand {
				t.Errorf("expected command %v, got %v and %v", wantCommand, command, done.Command)
			}
			if done.Verdict != testCase.wantVerdict || done.IsSkipped || done.Err != nil {
				t.Errorf("expected verdict %q, got %q (skipped %v, error %v)", testCase.wantVerdict, done.Verdict, done.IsSkipped, done.Err)
			}
		})
	}
}

// sequenceSteps runs the commands of a sequence message and returns their
// messages in order. Bubbletea does not export the type of a sequence message,
// so it is read as a slice of commands.
func sequenceSteps(t *testing.T, msg tea.Msg) []tea.Msg {
	t.Helper()
	value := reflect.ValueOf(msg)
	if value.Kind() != reflect.Slice {
		t.Fatalf("expected a sequence message, got %T", msg)
	}
	steps := make([]tea.Msg, 0, value.Len())
	for index := range value.Len() {
		step, ok := value.Index(index).Interface().(tea.Cmd)
		if !ok {
			t.Fatalf("expected a command at step %d of the sequence, got %T", index, value.Index(index).Interface())
		}
		steps = append(steps, step())
	}
	return steps
}
