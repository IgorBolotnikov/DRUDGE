package tasklist

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
		status   task.TaskStatus
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
		{name: "narrower", width: 40, height: 8, messages: []tea.Msg{loaded}},
		{name: "narrowest", width: 30, height: 8, messages: []tea.Msg{loaded}},
		{name: "taller_scrolls_back", width: 100, height: 8, messages: []tea.Msg{loaded, tuitest.KeyBottom, tea.WindowSizeMsg{Width: 100, Height: 3}, tea.WindowSizeMsg{Width: 100, Height: 8}}},
		{name: "filter_open", width: 100, height: 8, messages: []tea.Msg{loaded, tuitest.KeyFilter}},
		{name: "filtered", width: 100, height: 8, messages: slices.Concat([]tea.Msg{loaded, tuitest.KeyFilter}, tuitest.Type("RETR"))},
		{name: "filtered_no_match", width: 100, height: 8, messages: slices.Concat([]tea.Msg{loaded, tuitest.KeyFilter}, tuitest.Type("nothing"))},
		{name: "status_filter", width: 100, height: 8, status: task.StatusDone, messages: []tea.Msg{loaded}},
		{name: "status_filter_no_match", width: 100, height: 8, status: task.StatusUnmerged, messages: []tea.Msg{loaded}},
		{name: "filter_cleared", width: 100, height: 8, messages: slices.Concat([]tea.Msg{loaded, tuitest.KeyFilter}, tuitest.Type("retr"), []tea.Msg{tuitest.KeyEscape})},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles(), testCase.status)
			view.SetSize(testCase.width, testCase.height)
			for _, msg := range testCase.messages {
				if size, ok := msg.(tea.WindowSizeMsg); ok {
					view.SetSize(size.Width, size.Height)
					continue
				}
				view.Update(msg)
			}
			tuitest.AssertGolden(t, testCase.name, view.Render())
		})
	}
}

func TestView_TakesAllKeys(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: tuitest.ListedTasks()}
	cases := []struct {
		name     string
		messages []tea.Msg
		want     bool
	}{
		{name: "filter closed", messages: []tea.Msg{loaded}, want: false},
		{name: "filter open", messages: []tea.Msg{loaded, tuitest.KeyFilter}, want: true},
		{name: "filter closed with esc", messages: []tea.Msg{loaded, tuitest.KeyFilter, tuitest.KeyEscape}, want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles(), "")
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			if got := view.TakesAllKeys(); got != testCase.want {
				t.Errorf("expected takes all keys %t, got %t", testCase.want, got)
			}
		})
	}
}

func TestView_SelectsAndOpensTasks(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: tuitest.ListedTasks()}
	firstID := tuitest.ListedTasks()[0].Task.ID
	secondID := tuitest.ListedTasks()[1].Task.ID
	thirdID := tuitest.ListedTasks()[2].Task.ID
	cases := []struct {
		name     string
		messages []tea.Msg
		input    tea.Msg
		want     []tea.Msg
	}{
		{name: "cursor moves onto a task", messages: []tea.Msg{loaded}, input: tuitest.KeyDown, want: []tea.Msg{views.TaskSelectedMsg{TaskID: secondID}}},
		{name: "cursor stays at the top", messages: []tea.Msg{loaded}, input: tuitest.KeyUp},
		{name: "no tasks to move over", messages: []tea.Msg{views.TasksLoadedMsg{}}, input: tuitest.KeyDown},
		{name: "filter moves the cursor onto another task", messages: []tea.Msg{loaded, tuitest.KeyFilter}, input: tea.KeyPressMsg{Code: 'W', Text: "W"}, want: []tea.Msg{views.TaskSelectedMsg{TaskID: secondID}}},
		{name: "arrow moves the cursor while filtering", messages: []tea.Msg{loaded, tuitest.KeyFilter}, input: tuitest.KeyArrowDown, want: []tea.Msg{views.TaskSelectedMsg{TaskID: secondID}}},
		{name: "enter opens the task under the cursor", messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyDown}, input: tuitest.KeyEnter, want: []tea.Msg{views.OpenTaskMsg{TaskID: thirdID}}},
		{name: "enter opens the first task before the cursor moves", messages: []tea.Msg{loaded}, input: tuitest.KeyEnter, want: []tea.Msg{views.OpenTaskMsg{TaskID: firstID}}},
		{name: "enter opens a task while filtering", messages: slices.Concat([]tea.Msg{loaded, tuitest.KeyFilter}, tuitest.Type("backoff")), input: tuitest.KeyEnter, want: []tea.Msg{views.OpenTaskMsg{TaskID: secondID}}},
		{name: "enter with no task does nothing", messages: []tea.Msg{views.TasksLoadedMsg{}}, input: tuitest.KeyEnter},
		{name: "L opens the runlog of the task under the cursor", messages: []tea.Msg{loaded, tuitest.KeyDown}, input: tuitest.KeyRunLog, want: []tea.Msg{views.OpenRunLogMsg{TaskID: secondID}}},
		{name: "L with no task does nothing", messages: []tea.Msg{views.TasksLoadedMsg{}}, input: tuitest.KeyRunLog},
		{name: "L goes into the open filter", messages: []tea.Msg{loaded, tuitest.KeyFilter}, input: tuitest.KeyRunLog},
		{name: "r runs the task under the cursor", messages: []tea.Msg{loaded, tuitest.KeyDown}, input: tuitest.KeyRun, want: []tea.Msg{views.RunTaskMsg{TaskID: secondID}}},
		{name: "r with no task does nothing", messages: []tea.Msg{views.TasksLoadedMsg{}}, input: tuitest.KeyRun},
		{name: "r goes into the open filter", messages: []tea.Msg{loaded, tuitest.KeyFilter}, input: tuitest.KeyRun},
		{name: "m marks the task under the cursor done", messages: []tea.Msg{loaded, tuitest.KeyDown}, input: tuitest.KeyMarkDone, want: []tea.Msg{views.MarkDoneMsg{TaskID: secondID}}},
		{name: "m with no task does nothing", messages: []tea.Msg{views.TasksLoadedMsg{}}, input: tuitest.KeyMarkDone},
		{name: "click moves the cursor onto a task", messages: []tea.Msg{loaded}, input: tea.MouseClickMsg{Y: 3, Button: tea.MouseLeft}, want: []tea.Msg{views.TaskSelectedMsg{TaskID: thirdID}}},
		{name: "click on the cursor row", messages: []tea.Msg{loaded}, input: tea.MouseClickMsg{Y: 1, Button: tea.MouseLeft}},
		{name: "wheel moves the cursor onto a task", messages: []tea.Msg{loaded, tea.WindowSizeMsg{Width: 100, Height: 2}}, input: tea.MouseWheelMsg{Button: tea.MouseWheelDown}, want: []tea.Msg{views.TaskSelectedMsg{TaskID: secondID}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles(), "")
			view.SetSize(100, 8)
			for _, msg := range testCase.messages {
				if size, ok := msg.(tea.WindowSizeMsg); ok {
					view.SetSize(size.Width, size.Height)
					continue
				}
				view.Update(msg)
			}
			got := tuitest.Messages(view.Update(testCase.input))
			if !slices.Equal(got, testCase.want) {
				t.Errorf("expected messages %v, got %v", testCase.want, got)
			}
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
		{name: "filter open", messages: []tea.Msg{loaded, tuitest.KeyFilter}, want: "1/5"},
		{name: "filter ignores case", messages: slices.Concat([]tea.Msg{loaded, tuitest.KeyFilter}, tuitest.Type("RETR")), want: "1/2"},
		{name: "filter matches nothing", messages: slices.Concat([]tea.Msg{loaded, tuitest.KeyFilter}, tuitest.Type("nothing")), want: "0/0"},
		{name: "filter keeps the cursor on its task", messages: slices.Concat([]tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyDown, tuitest.KeyFilter}, tuitest.Type("retr")), want: "2/2"},
		{name: "filter cleared", messages: slices.Concat([]tea.Msg{loaded, tuitest.KeyFilter}, tuitest.Type("retr"), []tea.Msg{tuitest.KeyEscape}), want: "1/5"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles(), "")
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

// Parameters of the SGR sequences the nord theme sets on the cursor row.
const (
	nordSelectionBackground = "48;2;59;66;82"
	nordInfoForeground      = "38;2;136;192;208"
	nordMutedForeground     = "38;2;76;86;106"
	boldParameter           = "1"
)

var sgrPattern = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// styledRun is a piece of text with the SGR parameters in effect over it.
type styledRun struct {
	text       string
	parameters string
}

// styledRuns splits a line into runs of text and tracks the SGR parameters
// since the last reset, so a run shows what the terminal would draw.
func styledRuns(line string) []styledRun {
	var runs []styledRun
	parameters := ""
	rest := line
	for rest != "" {
		location := sgrPattern.FindStringSubmatchIndex(rest)
		if location == nil {
			runs = append(runs, styledRun{text: rest, parameters: parameters})
			break
		}
		if location[0] > 0 {
			runs = append(runs, styledRun{text: rest[:location[0]], parameters: parameters})
		}
		sequence := rest[location[2]:location[3]]
		if sequence == "" || sequence == "0" {
			parameters = ""
		} else {
			parameters += ";" + sequence
		}
		rest = rest[location[1]:]
	}
	return runs
}

func TestView_CursorRowStyle(t *testing.T) {
	const width = 100
	cases := []struct {
		name           string
		messages       []tea.Msg
		wantForeground map[string]string
		wantBold       string
	}{
		{
			name:           "a task keeps its status color and bold id",
			messages:       []tea.Msg{views.TasksLoadedMsg{Tasks: tuitest.ListedTasks()}},
			wantForeground: map[string]string{"in-progress": nordInfoForeground},
			wantBold:       "3f9a1c2e",
		},
		{
			name: "a context row keeps the muted color",
			messages: []tea.Msg{
				views.TasksLoadedMsg{Tasks: tuitest.ListedTasks()},
				tuitest.KeyFilter, tea.KeyPressMsg{Code: 'w', Text: "w"}, tea.KeyPressMsg{Code: 'r', Text: "r"}, tea.KeyPressMsg{Code: 'i', Text: "i"},
				tea.KeyPressMsg{Code: tea.KeyUp},
			},
			wantForeground: map[string]string{"Add retry": nordMutedForeground},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles(), "")
			view.SetSize(width, 8)
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			lines := strings.Split(view.Render(), "\n")
			var cursorLine string
			for _, line := range lines {
				if strings.Contains(line, cursorMarker) {
					cursorLine = line
				}
			}
			if cursorLine == "" {
				t.Fatalf("no cursor row in:\n%s", view.Render())
			}
			if got := ansi.StringWidth(cursorLine); got != width {
				t.Errorf("expected the cursor row to be %d cells wide, got %d", width, got)
			}
			for _, run := range styledRuns(cursorLine) {
				if !strings.Contains(run.parameters, nordSelectionBackground) {
					t.Errorf("expected the selection background over %q, got parameters %q", run.text, run.parameters)
				}
				for text, foreground := range testCase.wantForeground {
					if strings.Contains(run.text, text) && !strings.Contains(run.parameters, foreground) {
						t.Errorf("expected foreground %q over %q, got parameters %q", foreground, run.text, run.parameters)
					}
				}
				isBold := slices.Contains(strings.Split(run.parameters, ";"), boldParameter)
				if testCase.wantBold != "" && strings.Contains(run.text, testCase.wantBold) && !isBold {
					t.Errorf("expected %q to be bold, got parameters %q", run.text, run.parameters)
				}
			}
		})
	}
}

func TestView_ColumnsFollowTasks(t *testing.T) {
	const blockedByHeader = "BLOCKED BY"
	shortBlockers := tuitest.ListedTasks()
	for index := range shortBlockers {
		shortBlockers[index].Holding = nil
	}
	shortBlockers[0].Holding = []task.TaskID{"aaaaaaaa-0000"}
	longBlockers := tuitest.ListedTasks()
	for index := range longBlockers {
		longBlockers[index].Holding = nil
	}
	longBlockers[0].Holding = []task.TaskID{"aaaaaaaa-0000", "bbbbbbbb-0000", "cccccccc-0000"}

	view := New(tuitest.Styles(), "")
	view.SetSize(200, 8)
	view.Update(views.TasksLoadedMsg{Tasks: shortBlockers})
	shortWidth := blockedByWidth(t, view.Render(), blockedByHeader)
	view.Update(views.TasksLoadedMsg{Tasks: longBlockers})
	longWidth := blockedByWidth(t, view.Render(), blockedByHeader)
	if longWidth <= shortWidth {
		t.Errorf("expected the blockers column to widen from %d, got %d", shortWidth, longWidth)
	}
}

// blockedByWidth returns the cells between the header of the blockers column
// and the header of the next column.
func blockedByWidth(t *testing.T, rendered, blockedByHeader string) int {
	t.Helper()
	header := ansi.Strip(strings.SplitN(rendered, "\n", 2)[0])
	start := strings.Index(header, blockedByHeader)
	end := strings.Index(header, "TICKET")
	if start == -1 || end == -1 {
		t.Fatalf("expected the blockers and ticket headers in %q", header)
	}
	return end - start
}
