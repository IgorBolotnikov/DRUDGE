package taskdetail

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

var errPermissionDenied = errors.New("permission denied")

const shownTaskID task.TaskID = "3f9a1c2e-0000-0000-0000-000000000001"

const shownDescription = `# Retry uploads

The uploader gives up on the **first** network error. Retry it with a backoff.

- Retry up to 5 times
- Wait twice as long after each try
- Log every retry with the error that caused it

Run ` + "`go test ./internal/upload`" + ` before you finish.
`

// detailTasks returns a task with every field and a markdown description, and
// a task with no description.
func detailTasks() []task.ListedTask {
	created := time.Date(2026, time.November, 3, 9, 30, 0, 0, time.UTC)
	return []task.ListedTask{
		{Task: &task.Task{
			ID:           shownTaskID,
			Title:        "Add retry to uploader",
			Description:  shownDescription,
			Status:       task.StatusInProgress,
			TicketID:     "UP-12",
			ParentTaskID: "5b20d7aa-0000-0000-0000-000000000002",
			BlockedBy:    []task.TaskID{"7c1e4f90-0000-0000-0000-000000000003", "9d8b2a11-0000-0000-0000-000000000004"},
			Repositories: []string{"api"},
			CreatedAt:    created,
			StartedAt:    created.Add(time.Hour),
		}},
		{Task: &task.Task{ID: "a01f3b55-0000-0000-0000-000000000005", Title: "Draft the release notes", Status: task.StatusDraft}},
	}
}

// showMsg hands a task to the View in the messages of a test case.
type showMsg struct {
	taskID task.TaskID
}

func TestView(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: detailTasks()}
	cases := []struct {
		name     string
		width    int
		height   int
		messages []tea.Msg
	}{
		{name: "nothing_selected", width: 80, height: 10, messages: []tea.Msg{loaded}},
		{name: "loading", width: 80, height: 10, messages: []tea.Msg{showMsg{shownTaskID}}},
		{name: "gone", width: 80, height: 10, messages: []tea.Msg{loaded, showMsg{"ffffffff-0000"}}},
		{name: "wide", width: 100, height: 24, messages: []tea.Msg{loaded, showMsg{shownTaskID}}},
		{name: "narrow", width: 30, height: 24, messages: []tea.Msg{loaded, showMsg{shownTaskID}}},
		{name: "short_folds_fields", width: 100, height: 8, messages: []tea.Msg{loaded, showMsg{shownTaskID}}},
		{name: "scrolled", width: 100, height: 8, messages: []tea.Msg{loaded, showMsg{shownTaskID}, tuitest.KeyDown, tuitest.KeyDown}},
		{name: "scrolled_to_bottom", width: 100, height: 8, messages: []tea.Msg{loaded, showMsg{shownTaskID}, tuitest.KeyBottom}},
		{name: "reload_keeps_scroll", width: 100, height: 8, messages: []tea.Msg{loaded, showMsg{shownTaskID}, tuitest.KeyDown, loaded}},
		{name: "error_keeps_task", width: 100, height: 24, messages: []tea.Msg{loaded, showMsg{shownTaskID}, views.TasksLoadedMsg{Err: errPermissionDenied}}},
		{name: "no_description", width: 80, height: 10, messages: []tea.Msg{loaded, showMsg{"a01f3b55-0000-0000-0000-000000000005"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(testCase.width, testCase.height)
			for _, msg := range testCase.messages {
				if show, ok := msg.(showMsg); ok {
					view.ShowTask(show.taskID)
					continue
				}
				view.Update(msg)
			}
			tuitest.AssertGolden(t, testCase.name, view.Render())
		})
	}
}

func TestView_FitsItsSize(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "wide", width: 100, height: 24},
		{name: "narrow", width: 30, height: 24},
		{name: "short", width: 100, height: 8},
		{name: "one line", width: 60, height: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(testCase.width, testCase.height)
			view.Update(views.TasksLoadedMsg{Tasks: detailTasks()})
			view.ShowTask(shownTaskID)
			lines := strings.Split(view.Render(), "\n")
			if len(lines) > testCase.height {
				t.Errorf("expected at most %d lines, got %d", testCase.height, len(lines))
			}
		})
	}
}

func TestView_Status(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: detailTasks()}
	cases := []struct {
		name     string
		messages []tea.Msg
		want     string
	}{
		{name: "nothing selected", messages: []tea.Msg{loaded}, want: ""},
		{name: "task not loaded", messages: []tea.Msg{showMsg{shownTaskID}}, want: ""},
		{name: "at the top", messages: []tea.Msg{loaded, showMsg{shownTaskID}}, want: "3f9a1c2e 0%"},
		{name: "at the bottom", messages: []tea.Msg{loaded, showMsg{shownTaskID}, tuitest.KeyBottom}, want: "3f9a1c2e 100%"},
		{name: "another task starts at the top", messages: []tea.Msg{loaded, showMsg{shownTaskID}, tuitest.KeyBottom, showMsg{"a01f3b55-0000-0000-0000-000000000005"}}, want: "a01f3b55"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(100, 8)
			for _, msg := range testCase.messages {
				if show, ok := msg.(showMsg); ok {
					view.ShowTask(show.taskID)
					continue
				}
				view.Update(msg)
			}
			if got := view.Status(); got != testCase.want {
				t.Errorf("expected status %q, got %q", testCase.want, got)
			}
		})
	}
}
