package taskrunlog

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const shownTaskID task.TaskID = "3f9a1c2e-0000-0000-0000-000000000001"

const shownRunDir = ".drudge/runs/3f9a1c2e-0000-0000-0000-000000000001"

var errPermissionDenied = errors.New("permission denied")

var shownTask = &task.Task{ID: shownTaskID, Title: "Add retry to uploader", Status: task.StatusInProgress}

// Key presses that scroll the runlog besides the ones of the tuitest package.
var (
	keyArrowUp  = tea.KeyPressMsg{Code: tea.KeyUp}
	keyHalfUp   = tea.KeyPressMsg{Code: 'u', Text: "u"}
	keyHalfDown = tea.KeyPressMsg{Code: 'd', Text: "d"}
	keyPageUp   = tea.KeyPressMsg{Code: tea.KeyPgUp}
	keyPageDown = tea.KeyPressMsg{Code: tea.KeyPgDown}
	keyHome     = tea.KeyPressMsg{Code: tea.KeyHome}
	keyEnd      = tea.KeyPressMsg{Code: tea.KeyEnd}
	wheelUp     = tea.MouseWheelMsg{Button: tea.MouseWheelUp}
	wheelDown   = tea.MouseWheelMsg{Button: tea.MouseWheelDown}
)

// runRead returns a read of the run of the shown task with steps and stderr
// lines.
func runRead(steps []drudger.Step, stderr ...string) *drudger.RunStepsRead {
	return &drudger.RunStepsRead{Task: shownTask, RunDir: shownRunDir, HasRun: true, Harness: "claude", IsDecoded: true, Steps: steps, Stderr: stderr}
}

// firstRead returns the message of the first read of the feed.
func firstRead(read *drudger.RunStepsRead) views.RunStepsReadMsg {
	return views.RunStepsReadMsg{TaskID: shownTaskID, IsFirstRead: true, Read: read}
}

// laterRead returns the message of a read after the first one.
func laterRead(read *drudger.RunStepsRead) views.RunStepsReadMsg {
	return views.RunStepsReadMsg{TaskID: shownTaskID, Read: read}
}

// rerunRead returns the message of a read that found the task ran again.
func rerunRead(read *drudger.RunStepsRead) views.RunStepsReadMsg {
	read.IsReset = true
	return laterRead(read)
}

// textSteps returns one agent text step per summary.
func textSteps(summaries ...string) []drudger.Step {
	steps := make([]drudger.Step, 0, len(summaries))
	for _, summary := range summaries {
		steps = append(steps, drudger.Step{Kind: drudger.StepAgentText, Summary: summary})
	}
	return steps
}

// numberedSteps returns count agent text steps named by a prefix and their
// number.
func numberedSteps(prefix string, count int) []drudger.Step {
	summaries := make([]string, 0, count)
	for number := 1; number <= count; number++ {
		summaries = append(summaries, fmt.Sprintf("%s %d", prefix, number))
	}
	return textSteps(summaries...)
}

// everyKindOfStep returns a step of every kind, a multi-line text of a
// subagent among them.
func everyKindOfStep() []drudger.Step {
	return []drudger.Step{
		{Kind: drudger.StepInit, Summary: "session started in /workspace"},
		{Kind: drudger.StepAgentText, Summary: "I will add a retry to the uploader."},
		{Kind: drudger.StepToolCall, Tool: "Read", Summary: "internal/upload/upload.go"},
		{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go test ./internal/upload"},
		{Kind: drudger.StepToolError, Summary: "exit status 1"},
		{Kind: drudger.StepAgentText, Summary: "The subagent looks at the test.\nIt fails on slow machines.", Depth: 1},
		{Kind: drudger.StepFinalResult, Summary: "got-shit-done in 42s"},
	}
}

func TestView(t *testing.T) {
	noDecoder := runRead(nil)
	noDecoder.IsDecoded = false
	noDecoder.IsStreamMissing = true
	noRun := &drudger.RunStepsRead{Task: shownTask, RunDir: shownRunDir}
	cases := []struct {
		name     string
		width    int
		height   int
		messages []tea.Msg
	}{
		{name: "reading", width: 80, height: 10},
		{name: "read_error", width: 80, height: 10, messages: []tea.Msg{views.RunStepsReadMsg{TaskID: shownTaskID, IsFirstRead: true, Err: errPermissionDenied}}},
		{name: "no_run", width: 80, height: 10, messages: []tea.Msg{firstRead(noRun)}},
		{name: "run_starts", width: 80, height: 10, messages: []tea.Msg{firstRead(noRun), laterRead(runRead(textSteps("Reading the uploader")))}},
		{name: "every_kind_of_step", width: 80, height: 12, messages: []tea.Msg{firstRead(runRead(everyKindOfStep()))}},
		{name: "appends_steps", width: 80, height: 10, messages: []tea.Msg{firstRead(runRead(textSteps("first"))), laterRead(runRead(textSteps("second", "third")))}},
		{name: "error_keeps_steps", width: 80, height: 10, messages: []tea.Msg{firstRead(runRead(textSteps("first"))), views.RunStepsReadMsg{TaskID: shownTaskID, Err: errPermissionDenied}}},
		{name: "rerun_drops_steps", width: 80, height: 10, messages: []tea.Msg{firstRead(runRead(textSteps("old run"), "old stderr")), rerunRead(runRead(textSteps("new run")))}},
		{name: "stderr", width: 80, height: 10, messages: []tea.Msg{firstRead(runRead(textSteps("first"), "warning: slow disk")), laterRead(runRead(nil, "error: timeout"))}},
		{name: "no_decoder", width: 80, height: 10, messages: []tea.Msg{firstRead(noDecoder)}},
		{name: "first_read_replaces", width: 80, height: 10, messages: []tea.Msg{firstRead(runRead(textSteps("first"))), firstRead(runRead(textSteps("first", "second")))}},
		{name: "other_task_ignored", width: 80, height: 10, messages: []tea.Msg{firstRead(runRead(textSteps("first"))), views.RunStepsReadMsg{TaskID: "ffffffff", Read: runRead(textSteps("other"))}}},
		{name: "narrow_wraps_lines", width: 20, height: 20, messages: []tea.Msg{firstRead(runRead(everyKindOfStep()))}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(testCase.width, testCase.height)
			view.Open(views.OpenRunLogMsg{TaskID: shownTaskID})
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			tuitest.AssertGolden(t, testCase.name, view.Render())
		})
	}
}

func TestView_Follows(t *testing.T) {
	// The steps take 20 lines below the run directory in a View 5 lines tall.
	opened := firstRead(runRead(numberedSteps("step", 20)))
	cases := []struct {
		name       string
		messages   []tea.Msg
		wantStatus string
		// wantFirstLine is the first line the View shows.
		wantFirstLine string
	}{
		{name: "opens at the bottom", messages: []tea.Msg{opened}, wantStatus: followingStatus, wantFirstLine: "step 16"},
		{name: "k pauses", messages: []tea.Msg{opened, tuitest.KeyUp}, wantStatus: pausedStatus, wantFirstLine: "step 15"},
		{name: "up pauses", messages: []tea.Msg{opened, keyArrowUp}, wantStatus: pausedStatus, wantFirstLine: "step 15"},
		{name: "j back to the bottom follows", messages: []tea.Msg{opened, tuitest.KeyUp, tuitest.KeyDown}, wantStatus: followingStatus, wantFirstLine: "step 16"},
		{name: "down back to the bottom follows", messages: []tea.Msg{opened, keyArrowUp, tuitest.KeyArrowDown}, wantStatus: followingStatus, wantFirstLine: "step 16"},
		{name: "j short of the bottom stays paused", messages: []tea.Msg{opened, tuitest.KeyUp, tuitest.KeyUp, tuitest.KeyDown}, wantStatus: pausedStatus, wantFirstLine: "step 15"},
		{name: "u pauses", messages: []tea.Msg{opened, keyHalfUp}, wantStatus: pausedStatus, wantFirstLine: "step 14"},
		{name: "pgup pauses", messages: []tea.Msg{opened, keyPageUp}, wantStatus: pausedStatus, wantFirstLine: "step 14"},
		{name: "d back to the bottom follows", messages: []tea.Msg{opened, keyHalfUp, keyHalfDown}, wantStatus: followingStatus, wantFirstLine: "step 16"},
		{name: "pgdown back to the bottom follows", messages: []tea.Msg{opened, keyPageUp, keyPageDown}, wantStatus: followingStatus, wantFirstLine: "step 16"},
		{name: "g pauses at the top", messages: []tea.Msg{opened, tuitest.KeyTop}, wantStatus: pausedStatus, wantFirstLine: shownRunDir},
		{name: "home pauses at the top", messages: []tea.Msg{opened, keyHome}, wantStatus: pausedStatus, wantFirstLine: shownRunDir},
		{name: "G follows", messages: []tea.Msg{opened, tuitest.KeyTop, tuitest.KeyBottom}, wantStatus: followingStatus, wantFirstLine: "step 16"},
		{name: "end follows", messages: []tea.Msg{opened, keyHome, keyEnd}, wantStatus: followingStatus, wantFirstLine: "step 16"},
		{name: "wheel up pauses", messages: []tea.Msg{opened, wheelUp}, wantStatus: pausedStatus, wantFirstLine: "step 15"},
		{name: "wheel down back to the bottom follows", messages: []tea.Msg{opened, wheelUp, wheelDown}, wantStatus: followingStatus, wantFirstLine: "step 16"},
		{name: "new steps while following scroll to the bottom", messages: []tea.Msg{opened, laterRead(runRead(textSteps("step 21")))}, wantStatus: followingStatus, wantFirstLine: "step 17"},
		{name: "new steps while paused keep the scroll", messages: []tea.Msg{opened, tuitest.KeyUp, laterRead(runRead(textSteps("step 21")))}, wantStatus: pausedStatus, wantFirstLine: "step 15"},
		{name: "a rerun while following stays at the bottom", messages: []tea.Msg{opened, rerunRead(runRead(numberedSteps("rerun", 20)))}, wantStatus: followingStatus, wantFirstLine: "rerun 16"},
		{name: "a rerun while paused goes to the top", messages: []tea.Msg{opened, tuitest.KeyUp, rerunRead(runRead(numberedSteps("rerun", 20)))}, wantStatus: pausedStatus, wantFirstLine: shownRunDir},
		{name: "k on a run that fits keeps following", messages: []tea.Msg{firstRead(runRead(textSteps("step 1"))), tuitest.KeyUp}, wantStatus: followingStatus, wantFirstLine: shownRunDir},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(80, 5)
			view.Open(views.OpenRunLogMsg{TaskID: shownTaskID})
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			if got := view.Status(); got != testCase.wantStatus {
				t.Errorf("expected status %q, got %q", testCase.wantStatus, got)
			}
			firstLine, _, _ := strings.Cut(ansi.Strip(view.Render()), "\n")
			if got := strings.TrimSpace(firstLine); got != testCase.wantFirstLine {
				t.Errorf("expected the first line %q, got %q", testCase.wantFirstLine, got)
			}
		})
	}
}

// renderedLines returns the lines a View shows with their colors and trailing
// blanks stripped.
func renderedLines(view *View) []string {
	lines := strings.Split(ansi.Strip(view.Render()), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " ")
	}
	return lines
}

func TestView_Wraps(t *testing.T) {
	cases := []struct {
		name  string
		width int
		steps []drudger.Step
		// want is the lines the View shows below the run directory, which
		// fits on one line.
		want []string
	}{
		{
			name:  "plain text wraps at word boundaries",
			width: 20,
			steps: textSteps("I will add a retry to the uploader."),
			want:  []string{"I will add a retry", "to the uploader."},
		},
		{
			name:  "a word wider than the width breaks",
			width: 12,
			steps: textSteps("see internal/upload/upload.go"),
			want:  []string{"see", "internal/upl", "oad/upload.g", "o"},
		},
		{
			name:  "a tool call hangs under its summary",
			width: 44,
			steps: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go test ./internal/drudger/... -run TestRunStepsFrom -count=1"}},
			want:  []string{"Bash    go test ./internal/drudger/... -run", "        TestRunStepsFrom -count=1"},
		},
		{
			name:  "a flag keeps its dash",
			width: 40,
			steps: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go test ./internal/drudger/... -run TestRunStepsFrom"}},
			want:  []string{"Bash    go test ./internal/drudger/...", "        -run TestRunStepsFrom"},
		},
		{
			name:  "a tool error hangs under its message",
			width: 20,
			steps: []drudger.Step{{Kind: drudger.StepToolError, Summary: "exit status 1 after the timeout"}},
			want:  []string{"  ✗ exit status 1", "    after the", "    timeout"},
		},
		{
			name:  "a nested step keeps its depth indent",
			width: 20,
			steps: []drudger.Step{{Kind: drudger.StepAgentText, Summary: "The subagent looks at the test.", Depth: 2}},
			want:  []string{"    The subagent", "    looks at the", "    test."},
		},
		{
			name:  "a nested tool call hangs under its summary",
			width: 36,
			steps: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Read", Summary: "internal/upload/upload.go internal/upload/retry.go", Depth: 1}},
			want:  []string{"  Read    internal/upload/upload.go", "          internal/upload/retry.go"},
		},
		{
			name:  "every line of agent text wraps",
			width: 16,
			steps: textSteps("The test fails on slow machines.\nIt needs a longer timeout."),
			want:  []string{"The test fails", "on slow", "machines.", "It needs a", "longer timeout."},
		},
		{
			name:  "a hanging indent that leaves too little room is dropped",
			width: 14,
			steps: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go test ./internal/upload"}},
			want:  []string{"Bash    go", "test", "./internal/upl", "oad"},
		},
		{
			name:  "a width of one does not loop",
			width: 1,
			steps: textSteps("go"),
			want:  []string{"g", "o"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(testCase.width, 80)
			view.Open(views.OpenRunLogMsg{TaskID: shownTaskID})
			read := runRead(testCase.steps)
			read.RunDir = "r"
			view.Update(firstRead(read))
			got := slices.DeleteFunc(renderedLines(view)[1:], func(line string) bool { return line == "" })
			if !slices.Equal(got, testCase.want) {
				t.Errorf("expected lines %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestView_WrapKeepsColors(t *testing.T) {
	view := New(tuitest.Styles())
	view.SetSize(12, 10)
	view.Open(views.OpenRunLogMsg{TaskID: shownTaskID})
	read := runRead(nil, "error: timeout after 30s")
	read.RunDir = "r"
	view.Update(firstRead(read))
	// The run directory, a blank line and the stderr heading sit above the
	// three wrapped lines of stderr.
	stderrLines := strings.Split(view.Render(), "\n")[3:6]
	for _, line := range stderrLines {
		text := strings.TrimRight(ansi.Strip(line), " ")
		if want := view.styles.Error.Render(text); !strings.HasPrefix(line, want) {
			t.Errorf("expected the line %q to start with %q", line, want)
		}
	}
}

// resizeMsg sets the width of the View in a test of resizing.
type resizeMsg struct {
	width int
}

func TestView_Resizes(t *testing.T) {
	// Every step takes one line at 80 cells and two at 20 in a View 5 lines
	// tall.
	opened := firstRead(runRead(numberedSteps("summary that wraps, step", 20)))
	narrow := resizeMsg{width: 20}
	wide := resizeMsg{width: 80}
	cases := []struct {
		name          string
		messages      []tea.Msg
		wantStatus    string
		wantFirstLine string
		wantLastLine  string
	}{
		{name: "narrowing while following stays at the bottom", messages: []tea.Msg{opened, narrow}, wantStatus: followingStatus, wantFirstLine: "step 18", wantLastLine: "step 20"},
		{name: "widening while following stays at the bottom", messages: []tea.Msg{opened, narrow, wide}, wantStatus: followingStatus, wantFirstLine: "summary that wraps, step 16", wantLastLine: "summary that wraps, step 20"},
		{name: "narrowing while paused keeps the step at the top", messages: []tea.Msg{opened, tuitest.KeyUp, narrow}, wantStatus: pausedStatus, wantFirstLine: "summary that wraps,", wantLastLine: "summary that wraps,"},
		{name: "widening while paused keeps the step at the top", messages: []tea.Msg{opened, tuitest.KeyUp, narrow, wide}, wantStatus: pausedStatus, wantFirstLine: "summary that wraps, step 15", wantLastLine: "summary that wraps, step 19"},
		{name: "widening while paused inside a step keeps the step at the top", messages: []tea.Msg{opened, narrow, tuitest.KeyTop, tuitest.KeyDown, tuitest.KeyDown, tuitest.KeyDown, tuitest.KeyDown, wide}, wantStatus: pausedStatus, wantFirstLine: "summary that wraps, step 1", wantLastLine: "summary that wraps, step 5"},
		{name: "k pauses on wrapped lines", messages: []tea.Msg{opened, narrow, tuitest.KeyUp}, wantStatus: pausedStatus, wantFirstLine: "summary that wraps,", wantLastLine: "summary that wraps,"},
		{name: "j back to the bottom of wrapped lines follows", messages: []tea.Msg{opened, narrow, tuitest.KeyUp, tuitest.KeyDown}, wantStatus: followingStatus, wantFirstLine: "step 18", wantLastLine: "step 20"},
		{name: "new steps while following wrapped lines scroll to the bottom", messages: []tea.Msg{opened, narrow, laterRead(runRead(textSteps("summary that wraps, step 21")))}, wantStatus: followingStatus, wantFirstLine: "step 19", wantLastLine: "step 21"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(80, 5)
			view.Open(views.OpenRunLogMsg{TaskID: shownTaskID})
			for _, msg := range testCase.messages {
				if resize, ok := msg.(resizeMsg); ok {
					view.SetSize(resize.width, 5)
					continue
				}
				view.Update(msg)
			}
			if got := view.Status(); got != testCase.wantStatus {
				t.Errorf("expected status %q, got %q", testCase.wantStatus, got)
			}
			lines := renderedLines(view)
			if got := lines[0]; got != testCase.wantFirstLine {
				t.Errorf("expected the first line %q, got %q", testCase.wantFirstLine, got)
			}
			if got := lines[len(lines)-1]; got != testCase.wantLastLine {
				t.Errorf("expected the last line %q, got %q", testCase.wantLastLine, got)
			}
		})
	}
}

func TestView_Title(t *testing.T) {
	cases := []struct {
		name     string
		messages []tea.Msg
		want     string
	}{
		{name: "before the first read", want: "task-runlog [3f9a1c2e]"},
		{name: "after the first read", messages: []tea.Msg{firstRead(runRead(nil))}, want: "task-runlog [3f9a1c2e] Add retry to uploader"},
		{name: "after a failed read", messages: []tea.Msg{views.RunStepsReadMsg{TaskID: shownTaskID, IsFirstRead: true, Err: errPermissionDenied}}, want: "task-runlog [3f9a1c2e]"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.Open(views.OpenRunLogMsg{TaskID: shownTaskID})
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			if got := view.Title(); got != testCase.want {
				t.Errorf("expected title %q, got %q", testCase.want, got)
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
		wantFeeds   []views.Feed
	}{
		{name: "open runlog subscribes to the runlog of the task", msg: views.OpenRunLogMsg{TaskID: shownTaskID}, wantIsShown: true, wantFeeds: []views.Feed{{Name: views.RunLogFeed, TaskID: shownTaskID}}},
		{name: "open task is refused", msg: views.OpenTaskMsg{TaskID: shownTaskID}},
		{name: "another open message is refused", msg: unknownOpenMsg{}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			if got := view.Open(testCase.msg); got != testCase.wantIsShown {
				t.Errorf("expected shown %t, got %t", testCase.wantIsShown, got)
			}
			if got := view.Feeds(); !slices.Equal(got, testCase.wantFeeds) {
				t.Errorf("expected feeds %v, got %v", testCase.wantFeeds, got)
			}
		})
	}
}

func TestView_CommandsOnShownTask(t *testing.T) {
	cases := []struct {
		name   string
		taskID task.TaskID
		input  tea.Msg
		want   []tea.Msg
	}{
		{name: "r runs the shown task", taskID: shownTaskID, input: tuitest.KeyRun, want: []tea.Msg{views.RunTaskMsg{TaskID: shownTaskID}}},
		{name: "r with no task does nothing", input: tuitest.KeyRun},
		{name: "m marks the shown task done", taskID: shownTaskID, input: tuitest.KeyMarkDone, want: []tea.Msg{views.MarkDoneMsg{TaskID: shownTaskID}}},
		{name: "m with no task does nothing", input: tuitest.KeyMarkDone},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
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

func TestView_ShowTask(t *testing.T) {
	const otherTaskID task.TaskID = "5b20d7aa-0000-0000-0000-000000000002"
	cases := []struct {
		name          string
		messages      []tea.Msg
		taskID        task.TaskID
		wantFeeds     []views.Feed
		wantTitle     string
		wantStatus    string
		wantFirstLine string
	}{
		{
			name:          "another task drops the steps and swaps the feed",
			messages:      []tea.Msg{firstRead(runRead(numberedSteps("step", 20))), tuitest.KeyTop},
			taskID:        otherTaskID,
			wantFeeds:     []views.Feed{{Name: views.RunLogFeed, TaskID: otherTaskID}},
			wantTitle:     "task-runlog [5b20d7aa]",
			wantStatus:    followingStatus,
			wantFirstLine: "Reading the run…",
		},
		{
			name:          "the shown task keeps its steps",
			messages:      []tea.Msg{firstRead(runRead(numberedSteps("step", 20))), tuitest.KeyTop},
			taskID:        shownTaskID,
			wantFeeds:     []views.Feed{{Name: views.RunLogFeed, TaskID: shownTaskID}},
			wantTitle:     "task-runlog [3f9a1c2e] Add retry to uploader",
			wantStatus:    pausedStatus,
			wantFirstLine: shownRunDir,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(80, 5)
			view.ShowTask(shownTaskID)
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			view.ShowTask(testCase.taskID)
			if got := view.Feeds(); !slices.Equal(got, testCase.wantFeeds) {
				t.Errorf("expected feeds %v, got %v", testCase.wantFeeds, got)
			}
			if got := view.Title(); got != testCase.wantTitle {
				t.Errorf("expected title %q, got %q", testCase.wantTitle, got)
			}
			if got := view.Status(); got != testCase.wantStatus {
				t.Errorf("expected status %q, got %q", testCase.wantStatus, got)
			}
			firstLine, _, _ := strings.Cut(ansi.Strip(view.Render()), "\n")
			if got := strings.TrimSpace(firstLine); got != testCase.wantFirstLine {
				t.Errorf("expected the first line %q, got %q", testCase.wantFirstLine, got)
			}
		})
	}
}

func TestView_NoTaskSelected(t *testing.T) {
	view := New(tuitest.Styles())
	view.SetSize(80, 5)
	if got := view.Feeds(); got != nil {
		t.Errorf("expected no feeds, got %v", got)
	}
	tuitest.AssertGolden(t, "no_task_selected", view.Render())
}
