package taskboard

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

var errPermissionDenied = errors.New("permission denied")

// Key presses that move the cursor between columns.
var (
	keyLeft       = tea.KeyPressMsg{Code: 'h', Text: "h"}
	keyRight      = tea.KeyPressMsg{Code: 'l', Text: "l"}
	keyArrowRight = tea.KeyPressMsg{Code: tea.KeyRight}
)

// boardTasks returns the test tasks with two more tasks to do, so the todo
// column has three.
func boardTasks() []task.ListedTask {
	return append(tuitest.ListedTasks(),
		task.ListedTask{Task: &task.Task{ID: "b2c4d6e8-0000-0000-0000-000000000006", Title: "Document the retry settings", Status: task.StatusTodo}},
		task.ListedTask{Task: &task.Task{ID: "c3d5e7f9-0000-0000-0000-000000000007", Title: "Log every retry", Status: task.StatusTodo}},
	)
}

// todoTaskIDs returns the IDs of the tasks of the todo column, top first.
func todoTaskIDs() []task.TaskID {
	tasks := boardTasks()
	return []task.TaskID{tasks[2].Task.ID, tasks[5].Task.ID, tasks[6].Task.ID}
}

// update hands the View each message, and a window size as a new size.
func update(view *View, messages []tea.Msg) {
	for _, msg := range messages {
		if size, ok := msg.(tea.WindowSizeMsg); ok {
			view.SetSize(size.Width, size.Height)
			continue
		}
		view.Update(msg)
	}
}

func TestView(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: boardTasks()}
	cases := []struct {
		name     string
		width    int
		height   int
		messages []tea.Msg
	}{
		{name: "loading", width: 120, height: 5},
		{name: "error", width: 120, height: 5, messages: []tea.Msg{views.TasksLoadedMsg{Err: errPermissionDenied}}},
		{name: "error_keeps_tasks", width: 120, height: 5, messages: []tea.Msg{loaded, views.TasksLoadedMsg{Err: errPermissionDenied}}},
		{name: "empty", width: 120, height: 5, messages: []tea.Msg{views.TasksLoadedMsg{}}},
		{name: "wide", width: 120, height: 5, messages: []tea.Msg{loaded}},
		{name: "wide_moved", width: 120, height: 5, messages: []tea.Msg{loaded, keyRight, tuitest.KeyDown, tuitest.KeyDown}},
		{name: "wide_scrolled", width: 120, height: 3, messages: []tea.Msg{loaded, keyRight, tuitest.KeyBottom}},
		{name: "compact", width: 80, height: 10, messages: []tea.Msg{loaded}},
		{name: "compact_moved", width: 80, height: 10, messages: []tea.Msg{loaded, keyRight, tuitest.KeyDown}},
		{name: "compact_scrolled", width: 80, height: 8, messages: []tea.Msg{loaded, keyRight, tuitest.KeyBottom}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(testCase.width, testCase.height)
			update(view, testCase.messages)
			tuitest.AssertGolden(t, testCase.name, view.Render())
		})
	}
}

func TestView_Status(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: boardTasks()}
	reloadedWithoutCursorTask := views.TasksLoadedMsg{Tasks: boardTasks()[:6]}
	cases := []struct {
		name     string
		messages []tea.Msg
		want     string
	}{
		{name: "loading"},
		{name: "loaded", messages: []tea.Msg{loaded}, want: "draft 1/1"},
		{name: "no tasks", messages: []tea.Msg{views.TasksLoadedMsg{}}, want: "draft 0/0"},
		{name: "next column", messages: []tea.Msg{loaded, keyRight}, want: "todo 1/3"},
		{name: "arrow to the next column", messages: []tea.Msg{loaded, keyArrowRight}, want: "todo 1/3"},
		{name: "down the column", messages: []tea.Msg{loaded, keyRight, tuitest.KeyDown}, want: "todo 2/3"},
		{name: "bottom of the column", messages: []tea.Msg{loaded, keyRight, tuitest.KeyBottom}, want: "todo 3/3"},
		{name: "top of the column", messages: []tea.Msg{loaded, keyRight, tuitest.KeyBottom, tuitest.KeyTop}, want: "todo 1/3"},
		{name: "row moves up to the last row of a shorter column", messages: []tea.Msg{loaded, keyRight, tuitest.KeyBottom, keyRight}, want: "in-progress 1/1"},
		{name: "empty column", messages: []tea.Msg{loaded, keyRight, keyRight, keyRight, keyRight}, want: "unmerged 0/0"},
		{name: "stops at the first column", messages: []tea.Msg{loaded, keyLeft}, want: "draft 1/1"},
		{name: "stops at the last column", messages: slices.Concat([]tea.Msg{loaded}, slices.Repeat([]tea.Msg{keyRight}, 6)), want: "done 1/1"},
		{name: "reload keeps the cursor on its task", messages: []tea.Msg{loaded, keyRight, tuitest.KeyDown, views.TasksLoadedMsg{Tasks: boardTasks()[4:]}}, want: "todo 1/2"},
		{name: "reload without the cursor task keeps the row", messages: []tea.Msg{loaded, keyRight, tuitest.KeyBottom, reloadedWithoutCursorTask}, want: "todo 2/2"},
		{name: "click on a task", messages: []tea.Msg{loaded, tea.MouseClickMsg{X: 25, Y: 2, Button: tea.MouseLeft}}, want: "todo 2/3"},
		{name: "click on a header", messages: []tea.Msg{loaded, tea.MouseClickMsg{X: 110, Y: 0, Button: tea.MouseLeft}}, want: "done 1/1"},
		{name: "click below the last task", messages: []tea.Msg{loaded, tea.MouseClickMsg{X: 110, Y: 3, Button: tea.MouseLeft}}, want: "draft 1/1"},
		{name: "click on a compact header", messages: []tea.Msg{tea.WindowSizeMsg{Width: 80, Height: 10}, loaded, tea.MouseClickMsg{X: 5, Y: 2, Button: tea.MouseLeft}}, want: "todo 1/3"},
		{name: "click on a compact task", messages: []tea.Msg{tea.WindowSizeMsg{Width: 80, Height: 10}, loaded, keyRight, tea.MouseClickMsg{X: 5, Y: 4, Button: tea.MouseLeft}}, want: "todo 3/3"},
		{name: "wheel moves down the column", messages: []tea.Msg{loaded, keyRight, tea.MouseWheelMsg{Button: tea.MouseWheelDown}}, want: "todo 2/3"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(120, 5)
			update(view, testCase.messages)
			if got := view.Status(); got != testCase.want {
				t.Errorf("expected status %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestView_SelectsAndOpensTasks(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: boardTasks()}
	draftID := boardTasks()[4].Task.ID
	todoIDs := todoTaskIDs()
	cases := []struct {
		name     string
		messages []tea.Msg
		input    tea.Msg
		want     []tea.Msg
	}{
		{name: "cursor moves onto another column", messages: []tea.Msg{loaded}, input: keyRight, want: []tea.Msg{views.TaskSelectedMsg{TaskID: todoIDs[0]}}},
		{name: "cursor moves down a column", messages: []tea.Msg{loaded, keyRight}, input: tuitest.KeyDown, want: []tea.Msg{views.TaskSelectedMsg{TaskID: todoIDs[1]}}},
		{name: "cursor jumps to the bottom", messages: []tea.Msg{loaded, keyRight}, input: tuitest.KeyBottom, want: []tea.Msg{views.TaskSelectedMsg{TaskID: todoIDs[2]}}},
		{name: "cursor stays at the top", messages: []tea.Msg{loaded}, input: tuitest.KeyUp},
		{name: "cursor moves onto an empty column", messages: []tea.Msg{loaded, keyRight, keyRight, keyRight}, input: keyRight},
		{name: "click moves the cursor onto a task", messages: []tea.Msg{loaded}, input: tea.MouseClickMsg{X: 25, Y: 3, Button: tea.MouseLeft}, want: []tea.Msg{views.TaskSelectedMsg{TaskID: todoIDs[2]}}},
		{name: "enter opens the task under the cursor", messages: []tea.Msg{loaded, keyRight, tuitest.KeyDown}, input: tuitest.KeyEnter, want: []tea.Msg{views.OpenTaskMsg{TaskID: todoIDs[1]}}},
		{name: "enter opens the first task before the cursor moves", messages: []tea.Msg{loaded}, input: tuitest.KeyEnter, want: []tea.Msg{views.OpenTaskMsg{TaskID: draftID}}},
		{name: "enter in an empty column does nothing", messages: []tea.Msg{loaded, keyRight, keyRight, keyRight, keyRight}, input: tuitest.KeyEnter},
		{name: "enter with no tasks does nothing", messages: []tea.Msg{views.TasksLoadedMsg{}}, input: tuitest.KeyEnter},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(120, 5)
			update(view, testCase.messages)
			got := tuitest.Messages(view.Update(testCase.input))
			if !slices.Equal(got, testCase.want) {
				t.Errorf("expected messages %v, got %v", testCase.want, got)
			}
		})
	}
}

func TestView_CursorColumnHeader(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: boardTasks()}
	cases := []struct {
		name         string
		width        int
		messages     []tea.Msg
		wantReversed task.TaskStatus
	}{
		{name: "first column", width: 120, messages: []tea.Msg{loaded}, wantReversed: task.StatusDraft},
		{name: "moved column", width: 120, messages: []tea.Msg{loaded, keyRight}, wantReversed: task.StatusTodo},
		{name: "empty column", width: 120, messages: []tea.Msg{loaded, keyRight, keyRight, keyRight, keyRight}, wantReversed: task.StatusUnmerged},
		{name: "compact board", width: 80, messages: []tea.Msg{loaded, keyRight, keyRight}, wantReversed: task.StatusInProgress},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			screenStyles := tuitest.Styles()
			view := New(screenStyles)
			view.SetSize(testCase.width, 10)
			update(view, testCase.messages)
			rendered := view.Render()

			for _, status := range task.Statuses {
				reversedOpening, _, _ := strings.Cut(screenStyles.Status(status).Bold(true).Reverse(true).Render("x"), "x")
				isReversed := strings.Contains(rendered, reversedOpening+string(status)+" ")
				wantReversed := status == testCase.wantReversed
				if isReversed != wantReversed {
					t.Errorf("expected the header of %s reversed to be %v, got %v", status, wantReversed, isReversed)
				}
			}
		})
	}
}
