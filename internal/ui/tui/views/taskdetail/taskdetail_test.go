package taskdetail

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
		{name: "the wheel scrolls down", messages: []tea.Msg{loaded, showMsg{shownTaskID}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}}, want: "3f9a1c2e 50%"},
		{name: "the wheel scrolls back up", messages: []tea.Msg{loaded, showMsg{shownTaskID}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}, tea.MouseWheelMsg{Button: tea.MouseWheelUp}}, want: "3f9a1c2e 0%"},
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

func TestView_LabelColumnKeepsWidth(t *testing.T) {
	const wantValueCell = len("Pull requests") + len(labelGap)
	cases := []struct {
		name      string
		taskID    task.TaskID
		wantTitle string
	}{
		{name: "task with every field", taskID: shownTaskID, wantTitle: "Add retry to uploader"},
		{name: "task with few fields", taskID: "a01f3b55-0000-0000-0000-000000000005", wantTitle: "Draft the release notes"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(100, 24)
			view.Update(views.TasksLoadedMsg{Tasks: detailTasks()})
			view.ShowTask(testCase.taskID)

			titleLine, _, _ := strings.Cut(ansi.Strip(view.Render()), "\n")
			if got := strings.Index(titleLine, testCase.wantTitle); got != wantValueCell {
				t.Errorf("expected the title at cell %d, got it at %d in %q", wantValueCell, got, titleLine)
			}
		})
	}
}

func TestView_DescriptionGap(t *testing.T) {
	const fieldCount = 9
	cases := []struct {
		name         string
		height       int
		wantFolded   bool
		wantGapIndex int
	}{
		{name: "room for the fields, the gap and the description", height: fieldCount + descriptionGapLines + minDescriptionLines, wantGapIndex: fieldCount},
		{name: "one line short folds the fields with no gap", height: fieldCount + descriptionGapLines + minDescriptionLines - 1, wantFolded: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(100, testCase.height)
			view.Update(views.TasksLoadedMsg{Tasks: detailTasks()})
			view.ShowTask(shownTaskID)

			lines := strings.Split(ansi.Strip(view.Render()), "\n")
			isFolded := strings.Contains(lines[0], foldedSeparator)
			if isFolded != testCase.wantFolded {
				t.Fatalf("expected folded fields to be %v, got %v in %q", testCase.wantFolded, isFolded, lines[0])
			}
			if testCase.wantFolded {
				if strings.TrimSpace(lines[1]) == "" {
					t.Errorf("expected the description right below the folded fields, got a blank line")
				}
				return
			}
			if lines[testCase.wantGapIndex] != "" {
				t.Errorf("expected a blank line below the fields, got %q", lines[testCase.wantGapIndex])
			}
			if strings.TrimSpace(lines[testCase.wantGapIndex+1]) == "" {
				t.Errorf("expected the description right below the gap, got a blank line")
			}
		})
	}
}

// unknownOpenMsg is an open message the View can't show.
type unknownOpenMsg struct {
	views.OpenMsg
}

func TestView_Open(t *testing.T) {
	cases := []struct {
		name        string
		msg         views.OpenMsg
		wantIsShown bool
		wantStatus  string
	}{
		{name: "open task shows the task from the top", msg: views.OpenTaskMsg{TaskID: shownTaskID}, wantIsShown: true, wantStatus: "3f9a1c2e 0%"},
		{name: "another open message is refused", msg: unknownOpenMsg{}, wantIsShown: false, wantStatus: "a01f3b55"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(100, 8)
			view.Update(views.TasksLoadedMsg{Tasks: detailTasks()})
			view.ShowTask("a01f3b55-0000-0000-0000-000000000005")
			if got := view.Open(testCase.msg); got != testCase.wantIsShown {
				t.Errorf("expected shown %t, got %t", testCase.wantIsShown, got)
			}
			if got := view.Status(); got != testCase.wantStatus {
				t.Errorf("expected status %q, got %q", testCase.wantStatus, got)
			}
		})
	}
}

func TestView_TaskKeys(t *testing.T) {
	cases := []struct {
		name   string
		taskID task.TaskID
		input  tea.KeyPressMsg
		want   []tea.Msg
	}{
		{name: "L opens the runlog of the shown task", taskID: shownTaskID, input: tuitest.KeyRunLog, want: []tea.Msg{views.OpenRunLogMsg{TaskID: shownTaskID}}},
		{name: "L with no task does nothing", input: tuitest.KeyRunLog},
		{name: "r runs the shown task", taskID: shownTaskID, input: tuitest.KeyRun, want: []tea.Msg{views.RunTaskMsg{TaskID: shownTaskID}}},
		{name: "r with no task does nothing", input: tuitest.KeyRun},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(100, 8)
			view.Update(views.TasksLoadedMsg{Tasks: detailTasks()})
			if testCase.taskID != "" {
				view.ShowTask(testCase.taskID)
			}
			got := tuitest.Messages(view.Update(testCase.input))
			if !slices.Equal(got, testCase.want) {
				t.Errorf("expected messages %v, got %v", testCase.want, got)
			}
		})
	}
}
