package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

var (
	keyDown  = tea.KeyPressMsg{Code: 'j', Text: "j"}
	keyUp    = tea.KeyPressMsg{Code: 'k', Text: "k"}
	keyArrow = tea.KeyPressMsg{Code: tea.KeyDown}
	keyTop   = tea.KeyPressMsg{Code: 'g', Text: "g"}
	keyEnd   = tea.KeyPressMsg{Code: 'g', Text: "G", Mod: tea.ModShift}
)

func listedTasks() []task.ListedTask {
	parent := &task.Task{ID: "3f9a1c2e-0000-0000-0000-000000000001", Title: "Add retry to uploader", Status: task.StatusInProgress, TicketID: "UP-12"}
	return []task.ListedTask{
		{Task: parent},
		{Task: &task.Task{ID: "5b20d7aa-0000-0000-0000-000000000002", Title: "Write the backoff", Status: task.StatusDone, ParentTaskID: parent.ID}, IsUnderParent: true},
		{Task: &task.Task{ID: "7c1e4f90-0000-0000-0000-000000000003", Title: "Wire retries into the CLI", Status: task.StatusTodo}, Holding: []task.TaskID{parent.ID}},
		{Task: &task.Task{ID: "9d8b2a11-0000-0000-0000-000000000004", Title: "Fix the flaky upload test that fails on slow machines", Status: task.StatusFuckedUp, TicketID: "UP-7"}},
		{Task: &task.Task{ID: "a01f3b55-0000-0000-0000-000000000005", Title: "Draft the release notes", Status: task.StatusDraft}},
	}
}

func TestTaskListView(t *testing.T) {
	cases := []struct {
		name     string
		width    int
		height   int
		messages []tea.Msg
	}{
		{name: "task_list_loading", width: 80, height: 4},
		{name: "task_list_empty", width: 80, height: 4, messages: []tea.Msg{tasksLoadedMsg{}}},
		{name: "task_list_error", width: 80, height: 4, messages: []tea.Msg{tasksLoadedMsg{err: errors.New("permission denied")}}},
		{name: "task_list_loaded", width: 100, height: 8, messages: []tea.Msg{tasksLoadedMsg{tasks: listedTasks()}}},
		{name: "task_list_moved_down", width: 100, height: 8, messages: []tea.Msg{tasksLoadedMsg{tasks: listedTasks()}, keyDown, keyArrow, keyDown, keyUp}},
		{name: "task_list_scrolled_to_bottom", width: 100, height: 3, messages: []tea.Msg{tasksLoadedMsg{tasks: listedTasks()}, keyEnd, keyDown}},
		{name: "task_list_back_to_top", width: 100, height: 3, messages: []tea.Msg{tasksLoadedMsg{tasks: listedTasks()}, keyEnd, keyTop, keyUp}},
		{name: "task_list_narrow", width: 60, height: 8, messages: []tea.Msg{tasksLoadedMsg{tasks: listedTasks()}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := NewTaskListView(testStyles())
			view.SetSize(testCase.width, testCase.height)
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			assertGolden(t, testCase.name, view.View())
		})
	}
}
