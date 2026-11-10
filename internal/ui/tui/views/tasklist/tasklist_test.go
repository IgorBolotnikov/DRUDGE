package tasklist

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

var errPermissionDenied = errors.New("permission denied")

// reversedTasks returns the test tasks in reverse order.
func reversedTasks() []task.ListedTask {
	tasks := tuitest.ListedTasks()
	slices.Reverse(tasks)
	return tasks
}

func TestView(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: tuitest.ListedTasks()}
	failed := views.TasksLoadedMsg{Err: errPermissionDenied}
	cases := []struct {
		name     string
		width    int
		height   int
		messages []tea.Msg
	}{
		{name: "loading", width: 80, height: 4},
		{name: "empty", width: 80, height: 4, messages: []tea.Msg{views.TasksLoadedMsg{}}},
		{name: "error", width: 80, height: 4, messages: []tea.Msg{failed}},
		{name: "error_keeps_tasks", width: 100, height: 8, messages: []tea.Msg{loaded, tuitest.KeyDown, failed}},
		{name: "reload_keeps_cursor_on_task", width: 100, height: 8, messages: []tea.Msg{loaded, tuitest.KeyDown, views.TasksLoadedMsg{Tasks: reversedTasks()}}},
		{name: "reload_without_cursor_task", width: 100, height: 8, messages: []tea.Msg{loaded, tuitest.KeyBottom, views.TasksLoadedMsg{Tasks: tuitest.ListedTasks()[:2]}}},
		{name: "loaded", width: 100, height: 8, messages: []tea.Msg{loaded}},
		{name: "moved_down", width: 100, height: 8, messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyArrowDown, tuitest.KeyDown, tuitest.KeyUp}},
		{name: "scrolled_to_bottom", width: 100, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tuitest.KeyDown}},
		{name: "back_to_top", width: 100, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tuitest.KeyTop, tuitest.KeyUp}},
		{name: "narrow", width: 60, height: 8, messages: []tea.Msg{loaded}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(testCase.width, testCase.height)
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			tuitest.AssertGolden(t, testCase.name, view.Render())
		})
	}
}

func TestView_Status(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: tuitest.ListedTasks()}
	cases := []struct {
		name     string
		messages []tea.Msg
		want     string
	}{
		{name: "loading", want: ""},
		{name: "error before any tasks", messages: []tea.Msg{views.TasksLoadedMsg{Err: errPermissionDenied}}, want: ""},
		{name: "no tasks", messages: []tea.Msg{views.TasksLoadedMsg{}}, want: "0/0"},
		{name: "cursor on the first task", messages: []tea.Msg{loaded}, want: "1/5"},
		{name: "cursor moved", messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyDown}, want: "3/5"},
		{name: "error keeps the tasks", messages: []tea.Msg{loaded, tuitest.KeyBottom, views.TasksLoadedMsg{Err: errPermissionDenied}}, want: "5/5"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(100, 8)
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			if got := view.Status(); got != testCase.want {
				t.Errorf("expected status %q, got %q", testCase.want, got)
			}
		})
	}
}
