package statuspicker_test

import (
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/overlays/statuspicker"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	pickedTaskID        task.TaskID = "7c1e4f90-0000-0000-0000-000000000003"
	otherTaskID         task.TaskID = "3f9a1c2e-0000-0000-0000-000000000001"
	nordMutedForeground             = "38;2;76;86;106"
	cursorMarker                    = "› "
)

var helpKey = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help"))

// tasksLoaded is the tasks feed with the picked task at status.
func tasksLoaded(status task.TaskStatus) views.TasksLoadedMsg {
	return views.TasksLoadedMsg{Tasks: []task.ListedTask{
		{Task: &task.Task{ID: otherTaskID, Status: task.StatusDone}},
		{Task: &task.Task{ID: pickedTaskID, Status: status}},
	}}
}

// newPicker returns a picker that got each message.
func newPicker(messages []tea.Msg) *statuspicker.Picker {
	picker := statuspicker.New(pickedTaskID, helpKey, tuitest.Styles())
	for _, msg := range messages {
		picker.Update(msg)
	}
	return picker
}

// rows returns the rows of the box of picker with their cursor marker and the
// rows that are muted.
func rows(picker *statuspicker.Picker) (all, muted []string) {
	box, _, _ := picker.Render()
	for _, line := range strings.Split(box, "\n") {
		plain := strings.TrimSpace(strings.Trim(ansi.Strip(line), "│"))
		isStatusRow := slices.Contains(task.Statuses, task.TaskStatus(strings.TrimPrefix(plain, cursorMarker)))
		if !isStatusRow {
			continue
		}
		all = append(all, plain)
		if strings.Contains(line, nordMutedForeground) {
			muted = append(muted, plain)
		}
	}
	return all, muted
}

func cursorRow(picker *statuspicker.Picker) string {
	all, _ := rows(picker)
	for _, row := range all {
		if strings.HasPrefix(row, cursorMarker) {
			return strings.TrimPrefix(row, cursorMarker)
		}
	}
	return ""
}

func TestPicker_Cursor(t *testing.T) {
	cases := []struct {
		name     string
		messages []tea.Msg
		want     task.TaskStatus
	}{
		{name: "the cursor starts on the first status before the feed has the task", want: task.StatusDraft},
		{name: "the cursor moves onto the current status once the feed has the task", messages: []tea.Msg{tasksLoaded(task.StatusUnmerged)}, want: task.StatusUnmerged},
		{name: "a feed without the task leaves the cursor", messages: []tea.Msg{views.TasksLoadedMsg{}}, want: task.StatusDraft},
		{name: "j moves down", messages: []tea.Msg{tasksLoaded(task.StatusTodo), tuitest.KeyDown}, want: task.StatusInProgress},
		{name: "k moves up", messages: []tea.Msg{tasksLoaded(task.StatusTodo), tuitest.KeyUp}, want: task.StatusDraft},
		{name: "k stops at the first status", messages: []tea.Msg{tasksLoaded(task.StatusDraft), tuitest.KeyUp}, want: task.StatusDraft},
		{name: "j stops at the last status", messages: []tea.Msg{tasksLoaded(task.StatusDone), tuitest.KeyDown}, want: task.StatusDone},
		{name: "a later load keeps the cursor where it is", messages: []tea.Msg{tasksLoaded(task.StatusTodo), tuitest.KeyDown, tasksLoaded(task.StatusDone)}, want: task.StatusInProgress},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			picker := newPicker(testCase.messages)
			if got := cursorRow(picker); got != string(testCase.want) {
				t.Errorf("expected the cursor on %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestPicker_Apply(t *testing.T) {
	cases := []struct {
		name     string
		messages []tea.Msg
		wantNext tea.Msg
	}{
		{name: "the current status only closes the picker", messages: []tea.Msg{tasksLoaded(task.StatusTodo)}},
		{name: "another status sets it", messages: []tea.Msg{tasksLoaded(task.StatusDraft), tuitest.KeyDown}, wantNext: views.SetStatusMsg{TaskID: pickedTaskID, Status: task.StatusTodo}},
		{name: "done is set like any other status", messages: []tea.Msg{tasksLoaded(task.StatusUnmerged), tuitest.KeyDown}, wantNext: views.SetStatusMsg{TaskID: pickedTaskID, Status: task.StatusDone}},
		{name: "a status drudge maintains is forced", messages: []tea.Msg{tasksLoaded(task.StatusTodo), tuitest.KeyDown}, wantNext: views.SetStatusMsg{TaskID: pickedTaskID, Status: task.StatusInProgress, IsForced: true}},
		{name: "a status changed by a later load counts as current", messages: []tea.Msg{tasksLoaded(task.StatusDraft), tuitest.KeyDown, tasksLoaded(task.StatusTodo)}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			picker := newPicker(testCase.messages)
			cmd := picker.Update(tuitest.KeyEnter)
			if cmd == nil {
				t.Fatal("expected enter to close the picker, it sends nothing")
			}
			want := views.CloseOverlayMsg{Next: testCase.wantNext}
			if got := cmd(); got != want {
				t.Errorf("expected message %v, got %v", want, got)
			}
		})
	}
}

func TestPicker_Render(t *testing.T) {
	picker := newPicker([]tea.Msg{tasksLoaded(task.StatusTodo)})
	all, muted := rows(picker)

	wantRows := []string{"draft", "› todo", "in-progress", "fucked-up", "unmerged", "done"}
	if !slices.Equal(all, wantRows) {
		t.Errorf("expected rows %q, got %q", wantRows, all)
	}
	wantMuted := []string{"in-progress", "fucked-up", "unmerged"}
	if !slices.Equal(muted, wantMuted) {
		t.Errorf("expected muted rows %q, got %q", wantMuted, muted)
	}
}
