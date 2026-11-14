package tui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

var keyEventLog = tea.KeyPressMsg{Code: '!', Text: "!"}

var errNotTodo = errors.New(`task 7c1e4f90 is "in progress", only a task that is "todo" can run`)

// fakeCommandService refuses every run with runErr and records the task of
// every run. Its Drudgers hold the tasks it ran, from slot 2 up.
type fakeCommandService struct {
	runErr error
	runIDs []task.TaskID
}

func (service *fakeCommandService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	service.runIDs = append(service.runIDs, requestedID)
	return service.runErr
}

func (service *fakeCommandService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, errors.New("the command tests mark no task done")
}

func (service *fakeCommandService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	holders := make([]*drudger.Drudger, 0, len(service.runIDs))
	for index, runID := range service.runIDs {
		holders = append(holders, &drudger.Drudger{Slot: index + 2, TaskID: runID})
	}
	return holders, nil
}

func newTestCommands(service actions.DrudgerService) *actions.Actions {
	return actions.New(actions.Services{Drudgers: service}, testProject.Slug)
}

// startedTimer is one timer a model started.
type startedTimer struct {
	duration time.Duration
	fire     func(time.Time) tea.Msg
}

// recordTimers makes the timers of a model record themselves and never fire.
func recordTimers(model *Model) *[]startedTimer {
	var timers []startedTimer
	model.startTimer = func(duration time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
		timers = append(timers, startedTimer{duration: duration, fire: fire})
		return nil
	}
	return &timers
}

// newCommandTestModel returns a model of one task list Tab whose commands run
// on service.
func newCommandTestModel(t *testing.T, service *fakeCommandService) *Model {
	t.Helper()
	model := newTestModel(t, testTabs(t, "Tasks"), nil)
	model.commands = newTestCommands(service)
	update(model, tea.WindowSizeMsg{Width: 140, Height: 24})
	startFeeds(model)
	return model
}

// startHeld starts a command and returns the command that runs it without
// running it, so the command keeps running.
func startHeld(model *Model, msg views.CommandMsg) tea.Cmd {
	_, cmd := model.Update(msg)
	return cmd
}

func commandLine(model *Model) string {
	return ansi.Strip(model.renderCommandLine())
}

var (
	runTaskID      = tuitest.ListedTasks()[2].Task.ID
	otherRunTaskID = tuitest.ListedTasks()[4].Task.ID
)

func TestModel_RunsTaskOnKey(t *testing.T) {
	source := &fakeTaskSource{tasks: tuitest.ListedTasks()}
	model := newTestModelWithSource(t, source, testTabs(t, "Tasks"), nil)
	service := &fakeCommandService{}
	model.commands = newTestCommands(service)
	recordTimers(model)
	update(model, tea.WindowSizeMsg{Width: 140, Height: 24})
	startFeeds(model)
	listCountBefore := source.callCount.Load()

	pressKeys(model, []tea.Msg{tuitest.KeyDown, tuitest.KeyDown, tuitest.KeyRun})

	if !slices.Equal(service.runIDs, []task.TaskID{runTaskID}) {
		t.Errorf("expected one run of %s, got %v", runTaskID, service.runIDs)
	}
	if got, want := commandLine(model), "✓ 7c1e4f90 handed to Drudger 2"; got != want {
		t.Errorf("expected line %q, got %q", want, got)
	}
	if got := source.callCount.Load(); got != listCountBefore+1 {
		t.Errorf("expected the tasks feed to run once right after the command, got %d runs", got-listCountBefore)
	}
}

func TestModel_CommandLine(t *testing.T) {
	cases := []struct {
		name    string
		service *fakeCommandService
		// events are reported while the command runs.
		events []any
		// isDone is whether the command ends after its events.
		isDone bool
		// after are the messages that reach the model last.
		after    []tea.Msg
		wantLine string
	}{
		{name: "a command shows its first step", service: &fakeCommandService{}, wantLine: "⠋ 7c1e4f90: starting…"},
		{
			name:     "each step event replaces the step",
			service:  &fakeCommandService{},
			events:   []any{drudger.SandboxLookupStarted{Sandbox: "drudge-uploader-2"}, drudger.SandboxCreationStarted{Sandbox: "drudge-uploader-2", Timeout: 5 * time.Minute}},
			wantLine: "⠋ 7c1e4f90: Creating sandbox drudge-uploader-2, the first one pulls its image, up to 5m0s…",
		},
		{
			name:     "a warning leaves the step",
			service:  &fakeCommandService{},
			events:   []any{drudger.SandboxLookupStarted{Sandbox: "drudge-uploader-2"}, drudger.SbxDaemonRetried{}},
			wantLine: "⠋ 7c1e4f90: Looking for sandbox drudge-uploader-2…",
		},
		{
			name:     "a second command is refused while one runs",
			service:  &fakeCommandService{},
			after:    []tea.Msg{views.RunTaskMsg{TaskID: otherRunTaskID}},
			wantLine: "! wait, still running run 7c1e4f90",
		},
		{
			name:     "a step after a refusal replaces it",
			service:  &fakeCommandService{},
			after:    []tea.Msg{views.RunTaskMsg{TaskID: otherRunTaskID}, actions.ProgressMsg{Event: drudger.SandboxLookupStarted{Sandbox: "drudge-uploader-2"}}},
			wantLine: "⠋ 7c1e4f90: Looking for sandbox drudge-uploader-2…",
		},
		{name: "a run that worked shows its verdict", service: &fakeCommandService{}, isDone: true, wantLine: "✓ 7c1e4f90 handed to Drudger 2"},
		{
			name:     "the verdict counts the warnings",
			service:  &fakeCommandService{},
			events:   []any{drudger.BaseFetchFailed{Repository: "api", Branch: "main", Err: errDiskOnFire}, drudger.SbxDaemonRetried{}},
			isDone:   true,
			wantLine: "✓ 7c1e4f90 handed to Drudger 2 · 2 warnings, ! to see",
		},
		{name: "a refused run shows the error of the service", service: &fakeCommandService{runErr: errNotTodo}, isDone: true, wantLine: "✗ " + errNotTodo.Error()},
		{
			name:     "the error counts the warnings",
			service:  &fakeCommandService{runErr: errNotTodo},
			events:   []any{drudger.SbxDaemonRetried{}},
			isDone:   true,
			wantLine: "✗ " + errNotTodo.Error() + " · 1 warning, ! to see",
		},
		{name: "esc clears the error", service: &fakeCommandService{runErr: errNotTodo}, isDone: true, after: []tea.Msg{tuitest.KeyEscape}},
		{name: "other keys leave the error", service: &fakeCommandService{runErr: errNotTodo}, isDone: true, after: []tea.Msg{tuitest.KeyDown, keyNextTile}, wantLine: "✗ " + errNotTodo.Error()},
		{name: "the next command drops the error", service: &fakeCommandService{runErr: errNotTodo}, isDone: true, after: []tea.Msg{views.RunTaskMsg{TaskID: otherRunTaskID}}, wantLine: "⠋ a01f3b55: starting…"},
		{name: "esc leaves a verdict that worked", service: &fakeCommandService{}, isDone: true, after: []tea.Msg{tuitest.KeyEscape}, wantLine: "✓ 7c1e4f90 handed to Drudger 2"},
		{name: "events after the command ended are dropped", service: &fakeCommandService{}, isDone: true, after: []tea.Msg{actions.ProgressMsg{Event: drudger.SbxDaemonRetried{}}}, wantLine: "✓ 7c1e4f90 handed to Drudger 2"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newCommandTestModel(t, testCase.service)
			recordTimers(model)
			held := startHeld(model, views.RunTaskMsg{TaskID: runTaskID})
			for _, event := range testCase.events {
				update(model, actions.ProgressMsg{Event: event})
			}
			if testCase.isDone {
				runCommand(model, held)
			}
			for _, msg := range testCase.after {
				_, cmd := model.Update(msg)
				if _, isCommand := msg.(views.CommandMsg); !isCommand {
					runCommand(model, cmd)
				}
			}
			if got := commandLine(model); got != testCase.wantLine {
				t.Errorf("expected line %q, got %q", testCase.wantLine, got)
			}
		})
	}
}

func TestModel_RefusedCommandRunsNothing(t *testing.T) {
	service := &fakeCommandService{}
	model := newCommandTestModel(t, service)
	recordTimers(model)
	held := startHeld(model, views.RunTaskMsg{TaskID: runTaskID})

	update(model, views.RunTaskMsg{TaskID: otherRunTaskID})
	runCommand(model, held)

	if !slices.Equal(service.runIDs, []task.TaskID{runTaskID}) {
		t.Errorf("expected only the run of %s, got %v", runTaskID, service.runIDs)
	}
}

func TestModel_VerdictLifetime(t *testing.T) {
	cases := []struct {
		name       string
		service    *fakeCommandService
		wantTimers []time.Duration
	}{
		{name: "a verdict that worked clears after its lifetime", service: &fakeCommandService{}, wantTimers: []time.Duration{5 * time.Second}},
		{name: "an error stays", service: &fakeCommandService{runErr: errNotTodo}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newCommandTestModel(t, testCase.service)
			timers := recordTimers(model)
			update(model, views.RunTaskMsg{TaskID: runTaskID})

			var gotTimers []time.Duration
			for _, timer := range *timers {
				gotTimers = append(gotTimers, timer.duration)
			}
			if !slices.Equal(gotTimers, testCase.wantTimers) {
				t.Fatalf("expected timers %v, got %v", testCase.wantTimers, gotTimers)
			}
			if commandLine(model) == "" {
				t.Fatal("expected a verdict before its timer is up")
			}
			for _, timer := range *timers {
				update(model, timer.fire(time.Now()))
			}
			if len(testCase.wantTimers) > 0 && commandLine(model) != "" {
				t.Errorf("expected the verdict to clear, got %q", commandLine(model))
			}
		})
	}
}

func TestModel_OldTimerLeavesNewerVerdict(t *testing.T) {
	model := newCommandTestModel(t, &fakeCommandService{})
	timers := recordTimers(model)
	update(model, views.RunTaskMsg{TaskID: runTaskID})
	update(model, views.RunTaskMsg{TaskID: otherRunTaskID})

	update(model, (*timers)[0].fire(time.Now()))

	if got, want := commandLine(model), "✓ a01f3b55 handed to Drudger 3"; got != want {
		t.Errorf("expected line %q, got %q", want, got)
	}
}

func TestModel_EventLog(t *testing.T) {
	model := newCommandTestModel(t, &fakeCommandService{runErr: errNotTodo})
	recordTimers(model)
	held := startHeld(model, views.RunTaskMsg{TaskID: runTaskID})
	update(model, actions.ProgressMsg{Event: drudger.SandboxLookupStarted{Sandbox: "drudge-uploader-2"}})
	update(model, actions.ProgressMsg{Event: drudger.SbxDaemonRetried{}})
	runCommand(model, held)

	var gotEntries []string
	for _, entry := range model.events.entries {
		gotEntries = append(gotEntries, entry.command.String()+eventLogSeparator+entry.text)
	}
	wantEntries := []string{
		"run 7c1e4f90  The sbx daemon did not come up, DRUDGE gives it one more try",
		"run 7c1e4f90  " + errNotTodo.Error(),
	}
	if !slices.Equal(gotEntries, wantEntries) {
		t.Errorf("expected entries %q, got %q", wantEntries, gotEntries)
	}

	pressKeys(model, []tea.Msg{keyEventLog})
	if got := overlayTitles(model); !slices.Equal(got, []string{eventLogTitle}) {
		t.Fatalf("expected the event log open, got overlays %q", got)
	}
	screen := ansi.Strip(model.View().Content)
	for _, want := range wantEntries {
		if !strings.Contains(screen, want) {
			t.Errorf("expected the event log to show %q, got\n%s", want, screen)
		}
	}
	pressKeys(model, []tea.Msg{tuitest.KeyEscape})
	if got := overlayTitles(model); len(got) != 0 {
		t.Errorf("expected esc to close the event log, got overlays %q", got)
	}
}

func TestEventLog_KeepsNewestEntries(t *testing.T) {
	events := &eventLog{}
	for number := 1; number <= eventLogCap+1; number++ {
		events.add(eventLogEntry{text: strconv.Itoa(number)})
	}
	if got := len(events.entries); got != eventLogCap {
		t.Errorf("expected %d entries, got %d", eventLogCap, got)
	}
	if got := events.entries[0].text; got != "2" {
		t.Errorf("expected the oldest entry dropped, got first entry %q", got)
	}
}
