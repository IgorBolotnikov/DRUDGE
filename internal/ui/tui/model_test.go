package tui

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/drudgerlist"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/taskboard"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/taskdetail"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/taskrunlog"
)

var testProject = &project.Project{Slug: "uploader", Name: "Uploader"}

var errDiskOnFire = errors.New("disk on fire")

// Intervals of the feeds in the tests. A test that runs commands itself
// drops every tick, so its feeds tick right away. A feed of a running program
// ticks after the long interval, or several times during a test after the
// short one.
const (
	instantInterval = time.Nanosecond
	longInterval    = time.Hour
	shortInterval   = 10 * time.Millisecond
)

// waitTimeout caps how long a test waits for a run of a running program.
const waitTimeout = 5 * time.Second

// fakeTaskSource lists its tasks. The call numbered failFromCall and every
// later call fail with errDiskOnFire. A call sends its number on startedCalls
// and waits for release when they are set.
type fakeTaskSource struct {
	tasks        []task.ListedTask
	failFromCall int
	startedCalls chan int
	release      chan struct{}
	callCount    atomic.Int32
}

func (source *fakeTaskSource) ListTasks(projectSlug string, filter task.ListTasksFilter, page int, size int) (common.Page[task.ListedTask], error) {
	callNumber := int(source.callCount.Add(1))
	if source.startedCalls != nil {
		source.startedCalls <- callNumber
	}
	if source.release != nil {
		<-source.release
	}
	if source.failFromCall > 0 && callNumber >= source.failFromCall {
		return common.Page[task.ListedTask]{}, errDiskOnFire
	}
	return common.Paginate(source.tasks, page, size)
}

// fakeDrudgerSource reads and checks its Drudgers, checks the Sessions and
// reads a run of no steps of every test task. Each call sends its number on
// its started channel when it is set. A check waits for releaseChecks when it
// is set. A check fails with checkErr.
type fakeDrudgerSource struct {
	drudgers             []*drudger.Drudger
	checkErr             error
	startedReads         chan int
	startedChecks        chan int
	startedSessionChecks chan int
	releaseChecks        chan struct{}
	readCount            atomic.Int32
	checkCount           atomic.Int32
	sessionCheckCount    atomic.Int32
}

func (source *fakeDrudgerSource) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	readNumber := int(source.readCount.Add(1))
	if source.startedReads != nil {
		source.startedReads <- readNumber
	}
	return source.drudgers, nil
}

func (source *fakeDrudgerSource) ListDrudgers(projectSlug string, page int, size int) (common.Page[*drudger.Drudger], error) {
	checkNumber := int(source.checkCount.Add(1))
	if source.startedChecks != nil {
		source.startedChecks <- checkNumber
	}
	if source.releaseChecks != nil {
		<-source.releaseChecks
	}
	if source.checkErr != nil {
		return common.Page[*drudger.Drudger]{}, source.checkErr
	}
	return common.Paginate(source.drudgers, page, size)
}

func (source *fakeDrudgerSource) RecordFinishedSessions(projectSlug string) ([]task.TaskID, error) {
	checkNumber := int(source.sessionCheckCount.Add(1))
	if source.startedSessionChecks != nil {
		source.startedSessionChecks <- checkNumber
	}
	return nil, nil
}

func (source *fakeDrudgerSource) RunStepsFrom(projectSlug string, requestedID task.TaskID, cursor drudger.RunCursor) (*drudger.RunStepsRead, error) {
	for _, entry := range tuitest.ListedTasks() {
		if entry.Task.ID == requestedID {
			return &drudger.RunStepsRead{Task: entry.Task, RunDir: ".drudge/runs/" + string(requestedID), HasRun: true, IsDecoded: true}, nil
		}
	}
	return nil, fmt.Errorf("task %s not found", requestedID)
}

var (
	keyQuit = tea.KeyPressMsg{Code: 'q', Text: "q"}
	keyHelp = tea.KeyPressMsg{Code: '?', Text: "?"}
)

var (
	keyNextTab     = tea.KeyPressMsg{Code: ']', Text: "]"}
	keyPreviousTab = tea.KeyPressMsg{Code: '[', Text: "["}
	keyTabOne      = tea.KeyPressMsg{Code: '1', Text: "1"}
	keyTabTwo      = tea.KeyPressMsg{Code: '2', Text: "2"}
	keyTabThree    = tea.KeyPressMsg{Code: '3', Text: "3"}
	keyTabNine     = tea.KeyPressMsg{Code: '9', Text: "9"}
)

var (
	keyFocusLeft    = tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl}
	keyFocusDown    = tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}
	keyFocusUp      = tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	keyFocusRight   = tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl}
	keyArrowLeft    = tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl}
	keyArrowDown    = tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl}
	keyArrowUp      = tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl}
	keyArrowRight   = tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl}
	keyNextTile     = tea.KeyPressMsg{Code: tea.KeyTab}
	keyPreviousTile = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	keyZoom         = tea.KeyPressMsg{Code: 'z', Text: "z"}
)

// testTabs returns one task list Tab per title.
func testTabs(t *testing.T, titles ...string) []*Tab {
	t.Helper()
	config := Config{}
	for _, title := range titles {
		config.Tabs = append(config.Tabs, TabConfig{Title: title, Layout: NodeConfig{View: tasklist.Name}})
	}
	return buildTestTabs(t, config)
}

// buildTestTabs builds the Tabs of a config with the test styles.
func buildTestTabs(t *testing.T, config Config) []*Tab {
	t.Helper()
	tabs, err := BuildTabs(config, tuitest.Styles())
	if err != nil {
		t.Fatal(err)
	}
	return tabs
}

// newTestModel returns a Model of the test project whose source lists the test
// tasks.
func newTestModel(t *testing.T, tabs []*Tab, warnings []string) *Model {
	t.Helper()
	return newTestModelWithSource(t, &fakeTaskSource{tasks: tuitest.ListedTasks()}, tabs, warnings)
}

// newTestModelWithSource returns a Model of the test project that lists the
// tasks of source and has no Drudgers.
func newTestModelWithSource(t *testing.T, source feeds.TaskSource, tabs []*Tab, warnings []string) *Model {
	t.Helper()
	sources := feeds.Sources{Tasks: source, Drudgers: &fakeDrudgerSource{}}
	return newTestModelWithSources(t, sources, tabs, intervalsOf(instantInterval), warnings)
}

func newTestModelWithSources(t *testing.T, sources feeds.Sources, tabs []*Tab, intervals map[views.FeedName]time.Duration, warnings []string) *Model {
	t.Helper()
	model, err := NewModel(testProject, sources, newTestCommands(&fakeCommandService{}), tuitest.Styles(), tabs, intervals, true, warnings)
	if err != nil {
		t.Fatal(err)
	}
	return model
}

// intervalsOf returns one interval for every feed.
func intervalsOf(interval time.Duration) map[views.FeedName]time.Duration {
	intervals := map[views.FeedName]time.Duration{}
	for _, kind := range feedKinds {
		intervals[kind.Name] = interval
	}
	return intervals
}

// startFeeds starts the feeds of the visible Views and runs their first runs.
// It starts nothing after an update started the feeds.
func startFeeds(model *Model) {
	runCommand(model, model.Init())
}

// update sends a message to the model and runs the commands it leads to.
func update(model *Model, msg tea.Msg) {
	_, cmd := model.Update(msg)
	runCommand(model, cmd)
}

// runCommand runs a command and every command its messages lead to. It drops
// the ticks of the feeds, so each feed runs once when it starts, and the ticks
// of the spinner.
func runCommand(model *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, batched := range msg {
			runCommand(model, batched)
		}
	case feeds.TickMsg, spinner.TickMsg:
	default:
		_, next := model.Update(msg)
		runCommand(model, next)
	}
}

// startProgram runs a model in a program with no terminal and quits it when
// the test ends.
func startProgram(t *testing.T, model tea.Model) *tea.Program {
	t.Helper()
	program := tea.NewProgram(model,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
		tea.WithoutSignals(),
		tea.WithWindowSize(100, 12),
	)
	isDone := make(chan struct{})
	go func() {
		defer close(isDone)
		if _, err := program.Run(); err != nil {
			t.Errorf("the program failed: %v", err)
		}
	}()
	t.Cleanup(func() {
		program.Quit()
		<-isDone
	})
	return program
}

// waitForCall waits until the source starts the call numbered wantNumber.
func waitForCall(t *testing.T, startedCalls chan int, wantNumber int) {
	t.Helper()
	timeout := time.After(waitTimeout)
	for {
		select {
		case number := <-startedCalls:
			if number == wantNumber {
				return
			}
		case <-timeout:
			t.Fatalf("the source did not start call %d within %s", wantNumber, waitTimeout)
		}
	}
}

func TestModel(t *testing.T) {
	cases := []struct {
		name      string
		width     int
		height    int
		tabTitles []string
		warnings  []string
		keys      []tea.KeyPressMsg
	}{
		{name: "screen", width: 100, height: 12},
		{name: "screen_cursor_moved", width: 100, height: 12, keys: []tea.KeyPressMsg{tuitest.KeyDown, tuitest.KeyDown}},
		{name: "screen_with_warning", width: 100, height: 12, warnings: []string{`theme.json: "red" is not a color for role error, using the theme's own`}},
		{name: "screen_short", width: 100, height: 5},
		{name: "screen_narrow", width: 30, height: 8},
		{name: "screen_tabs", width: 100, height: 12, tabTitles: []string{"Tasks", "Board", "Drudgers"}},
		{name: "screen_tabs_cursor_on_second", width: 100, height: 12, tabTitles: []string{"Tasks", "Board"}, keys: []tea.KeyPressMsg{keyNextTab, tuitest.KeyDown, keyPreviousTab}},
		{name: "screen_tabs_scrolled", width: 30, height: 8, tabTitles: []string{"Tasks", "Board", "Drudgers", "Backlog"}, keys: []tea.KeyPressMsg{keyPreviousTab}},
		{name: "screen_too_small_one_line", width: 60, height: 3},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tabTitles := testCase.tabTitles
			if tabTitles == nil {
				tabTitles = []string{"Tasks"}
			}
			model := newTestModel(t, testTabs(t, tabTitles...), testCase.warnings)
			update(model, tea.WindowSizeMsg{Width: testCase.width, Height: testCase.height})
			startFeeds(model)
			for _, keyPress := range testCase.keys {
				model.Update(keyPress)
			}

			view := model.View()
			if !view.AltScreen {
				t.Error("expected the screen in the alt screen")
			}
			if wantTitle := "DRUDGE | Uploader"; view.WindowTitle != wantTitle {
				t.Errorf("expected window title %q, got %q", wantTitle, view.WindowTitle)
			}
			lines := strings.Split(view.Content, "\n")
			if len(lines) != testCase.height {
				t.Errorf("expected %d lines, got %d", testCase.height, len(lines))
			}
			tuitest.AssertGolden(t, testCase.name, view.Content)
		})
	}
}

func TestModel_Quits(t *testing.T) {
	cases := []struct {
		name     string
		keyPress tea.KeyPressMsg
	}{
		{name: "q", keyPress: tea.KeyPressMsg{Code: 'q', Text: "q"}},
		{name: "ctrl+c", keyPress: tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, testTabs(t, "Tasks"), nil)
			startFeeds(model)
			_, cmd := model.Update(testCase.keyPress)
			if cmd == nil {
				t.Fatal("expected a command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Error("expected the command to quit")
			}
		})
	}
}

func TestModel_SwitchesTabs(t *testing.T) {
	cases := []struct {
		name          string
		keys          []tea.KeyPressMsg
		wantActiveTab string
	}{
		{name: "first tab at start", wantActiveTab: "Tasks"},
		{name: "next tab", keys: []tea.KeyPressMsg{keyNextTab}, wantActiveTab: "Board"},
		{name: "next tab wraps to the first", keys: []tea.KeyPressMsg{keyNextTab, keyNextTab, keyNextTab}, wantActiveTab: "Tasks"},
		{name: "previous tab wraps to the last", keys: []tea.KeyPressMsg{keyPreviousTab}, wantActiveTab: "Drudgers"},
		{name: "number key", keys: []tea.KeyPressMsg{keyTabThree}, wantActiveTab: "Drudgers"},
		{name: "number key then previous tab", keys: []tea.KeyPressMsg{keyTabThree, keyPreviousTab}, wantActiveTab: "Board"},
		{name: "number key with no tab", keys: []tea.KeyPressMsg{keyTabTwo, keyTabNine}, wantActiveTab: "Board"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, testTabs(t, "Tasks", "Board", "Drudgers"), nil)
			startFeeds(model)
			for _, keyPress := range testCase.keys {
				if _, cmd := model.Update(keyPress); cmd != nil {
					t.Errorf("expected no command from %q", keyPress.String())
				}
			}
			if got := model.ActiveTab().Title; got != testCase.wantActiveTab {
				t.Errorf("expected tab %q active, got %q", testCase.wantActiveTab, got)
			}
		})
	}
}

func TestModel_Filter(t *testing.T) {
	cases := []struct {
		name          string
		keys          []tea.Msg
		wantActiveTab string
		wantStatuses  []string
		wantQuit      bool
		golden        string
	}{
		{name: "matching ignores case", keys: slices.Concat([]tea.Msg{tuitest.KeyFilter}, tuitest.Type("RETR")), wantActiveTab: "Tasks", wantStatuses: []string{"1/2"}, golden: "screen_filtered"},
		{name: "a digit goes to the filter", keys: slices.Concat([]tea.Msg{tuitest.KeyFilter}, tuitest.Type("2")), wantActiveTab: "Tasks", wantStatuses: []string{"0/0"}},
		{name: "root keys go to the filter", keys: []tea.Msg{tuitest.KeyFilter, keyQuit, keyNextTab, keyPreviousTab, keyZoom}, wantActiveTab: "Tasks", wantStatuses: []string{"0/0"}},
		{name: "ctrl+c quits from the filter", keys: []tea.Msg{tuitest.KeyFilter, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}}, wantActiveTab: "Tasks", wantStatuses: []string{"1/5"}, wantQuit: true},
		{name: "esc restores the full list", keys: slices.Concat([]tea.Msg{tuitest.KeyFilter}, tuitest.Type("RETR"), []tea.Msg{tuitest.KeyEscape}), wantActiveTab: "Tasks", wantStatuses: []string{"1/5"}},
		{name: "a digit switches tabs after esc", keys: []tea.Msg{tuitest.KeyFilter, tuitest.KeyEscape, keyTabTwo}, wantActiveTab: "Board", wantStatuses: []string{"1/5"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, testTabs(t, "Tasks", "Board"), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
			startFeeds(model)
			didQuit := false
			for _, msg := range testCase.keys {
				_, cmd := model.Update(msg)
				if isQuit(cmd) {
					didQuit = true
				}
			}
			if didQuit != testCase.wantQuit {
				t.Errorf("expected quit %t, got %t", testCase.wantQuit, didQuit)
			}
			if got := model.ActiveTab().Title; got != testCase.wantActiveTab {
				t.Errorf("expected tab %q active, got %q", testCase.wantActiveTab, got)
			}
			if got := tileStatuses(model.ActiveTab()); !slices.Equal(got, testCase.wantStatuses) {
				t.Errorf("expected statuses %q, got %q", testCase.wantStatuses, got)
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, model.View().Content)
			}
		})
	}
}

func TestModel_HelpOverlay(t *testing.T) {
	cases := []struct {
		name          string
		width         int
		height        int
		keys          []tea.Msg
		wantOverlays  []string
		wantActiveTab string
		golden        string
	}{
		{name: "help opens", width: 100, height: 24, keys: []tea.Msg{keyHelp}, wantOverlays: []string{helpOverlayTitle}, wantActiveTab: "First", golden: "screen_help"},
		{name: "help in a terminal smaller than the box", width: 30, height: 8, keys: []tea.Msg{keyHelp}, wantOverlays: []string{helpOverlayTitle}, wantActiveTab: "First", golden: "screen_help_too_small"},
		{name: "help titles the keys of the focused view with the title of its tile", width: 100, height: 24, keys: []tea.Msg{keyNextTile, keyHelp}, wantOverlays: []string{helpOverlayTitle}, wantActiveTab: "First", golden: "screen_help_tile_title"},
		{name: "esc closes help", width: 100, height: 24, keys: []tea.Msg{keyHelp, tuitest.KeyEscape}, wantActiveTab: "First"},
		{name: "help key closes help", width: 100, height: 24, keys: []tea.Msg{keyHelp, keyHelp}, wantActiveTab: "First"},
		{
			name:          "help takes the root and view keys",
			width:         100,
			height:        24,
			keys:          []tea.Msg{keyHelp, keyNextTab, keyPreviousTab, keyTabTwo, keyZoom, keyNextTile, tuitest.KeyDown, tuitest.KeyFilter},
			wantOverlays:  []string{helpOverlayTitle},
			wantActiveTab: "First",
			golden:        "screen_help_takes_keys",
		},
		{name: "the help key goes to an open filter", width: 100, height: 24, keys: []tea.Msg{tuitest.KeyFilter, keyHelp}, wantActiveTab: "First"},
		{name: "keys work again after help closes", width: 100, height: 24, keys: []tea.Msg{keyHelp, tuitest.KeyEscape, keyTabTwo}, wantActiveTab: "Second"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := Config{Tabs: []TabConfig{
				oneTabConfig(NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), {View: tasklist.Name, Title: "Backlog"}}}).Tabs[0],
				{Title: "Second", Layout: taskListNode()},
			}}
			config.Tabs[0].Title = "First"
			model := newTestModel(t, buildTestTabs(t, config), nil)
			update(model, tea.WindowSizeMsg{Width: testCase.width, Height: testCase.height})
			startFeeds(model)
			for _, msg := range testCase.keys {
				_, cmd := model.Update(msg)
				if isQuit(cmd) {
					t.Fatalf("expected no quit from %v", msg)
				}
			}
			if got := overlayTitles(model); !slices.Equal(got, testCase.wantOverlays) {
				t.Errorf("expected overlays %q, got %q", testCase.wantOverlays, got)
			}
			if got := model.ActiveTab().Title; got != testCase.wantActiveTab {
				t.Errorf("expected tab %q active, got %q", testCase.wantActiveTab, got)
			}
			content := model.View().Content
			if lines := strings.Split(content, "\n"); len(lines) != testCase.height {
				t.Errorf("expected %d lines, got %d", testCase.height, len(lines))
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, content)
			}
		})
	}
}

// overlayTitles returns the titles of the open overlays from the bottom of the
// stack up.
func overlayTitles(model *Model) []string {
	var titles []string
	for _, overlay := range model.overlays {
		titles = append(titles, overlay.Title())
	}
	return titles
}

// pressKeys presses each key and runs the commands it starts.
func pressKeys(model *Model, keys []tea.Msg) {
	for _, keyPress := range keys {
		_, cmd := model.Update(keyPress)
		runCommand(model, cmd)
	}
}

// listAndDetailTab describes a Tab with a task list beside a task detail.
func listAndDetailTab(title string) TabConfig {
	return TabConfig{Title: title, Layout: NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{
		taskListNode(),
		{View: taskdetail.Name},
	}}}
}

func TestModel_SelectionStaysInItsTab(t *testing.T) {
	secondID := tuitest.ListedTasks()[1].Task.ID
	thirdID := tuitest.ListedTasks()[2].Task.ID
	cases := []struct {
		name              string
		keys              []tea.Msg
		wantFirstTabTask  task.TaskID
		wantSecondTabTask task.TaskID
		golden            string
	}{
		{name: "nothing selected at start", golden: "screen_nothing_selected"},
		{name: "cursor selects in its tab", keys: []tea.Msg{tuitest.KeyDown}, wantFirstTabTask: secondID, golden: "screen_task_selected"},
		{name: "other tab keeps nothing selected", keys: []tea.Msg{tuitest.KeyDown, keyTabTwo}, wantFirstTabTask: secondID, golden: "screen_selection_in_other_tab"},
		{name: "each tab keeps its own selection", keys: []tea.Msg{tuitest.KeyDown, keyTabTwo, tuitest.KeyDown, tuitest.KeyDown, keyTabOne}, wantFirstTabTask: secondID, wantSecondTabTask: thirdID},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := Config{Tabs: []TabConfig{listAndDetailTab("First"), listAndDetailTab("Second")}}
			model := newTestModel(t, buildTestTabs(t, config), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
			startFeeds(model)
			pressKeys(model, testCase.keys)
			if got := model.tabs[0].SelectedTaskID(); got != testCase.wantFirstTabTask {
				t.Errorf("expected task %q selected in the first tab, got %q", testCase.wantFirstTabTask, got)
			}
			if got := model.tabs[1].SelectedTaskID(); got != testCase.wantSecondTabTask {
				t.Errorf("expected task %q selected in the second tab, got %q", testCase.wantSecondTabTask, got)
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, model.View().Content)
			}
		})
	}
}

func TestModel_BoardDrivesTaskDetail(t *testing.T) {
	keyNextColumn := tea.KeyPressMsg{Code: 'l', Text: "l"}
	todoID := tuitest.ListedTasks()[2].Task.ID
	inProgressID := tuitest.ListedTasks()[0].Task.ID
	cases := []struct {
		name     string
		keys     []tea.Msg
		wantTask task.TaskID
		golden   string
	}{
		{name: "nothing selected at start", golden: "screen_board_nothing_selected"},
		{name: "next column selects its task", keys: []tea.Msg{keyNextColumn}, wantTask: todoID, golden: "screen_board_task_selected"},
		{name: "each move selects the task under the cursor", keys: []tea.Msg{keyNextColumn, keyNextColumn}, wantTask: inProgressID},
		{name: "enter opens the task under the cursor", keys: []tea.Msg{tuitest.KeyEnter}, wantTask: tuitest.ListedTasks()[4].Task.ID},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := oneTabConfig(NodeConfig{Arrangement: Stacked, Children: []NodeConfig{{View: taskboard.Name}, {View: taskdetail.Name}}})
			model := newTestModel(t, buildTestTabs(t, config), nil)
			update(model, tea.WindowSizeMsg{Width: 120, Height: 16})
			startFeeds(model)
			pressKeys(model, testCase.keys)
			detail := model.ActiveTab().tiles[1].view.(*taskdetail.View)
			if got := detail.Status(); got != task.ShortID(testCase.wantTask) {
				t.Errorf("expected the task detail to show task %q, got status %q", testCase.wantTask, got)
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, model.View().Content)
			}
		})
	}
}

func TestModel_OpensTask(t *testing.T) {
	listAndDetail := oneTabConfig(NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), {View: taskdetail.Name}}})
	twoLists := oneTabConfig(NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), taskListNode()}})
	twoLists.Tabs = append(twoLists.Tabs, TabConfig{Title: "Second", Layout: taskListNode()})
	cases := []struct {
		name                 string
		config               Config
		keys                 []tea.Msg
		wantOverlays         []string
		wantFocusedTileIndex int
		wantActiveTab        string
		golden               string
	}{
		{name: "enter focuses the task detail", config: listAndDetail, keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyDown, tuitest.KeyEnter}, wantFocusedTileIndex: 1, wantActiveTab: "Tab", golden: "screen_task_opened_in_tile"},
		{name: "esc in the task detail keeps focus", config: listAndDetail, keys: []tea.Msg{tuitest.KeyEnter, tuitest.KeyEscape}, wantFocusedTileIndex: 1, wantActiveTab: "Tab"},
		{name: "enter opens the overlay without a task detail", config: twoLists, keys: []tea.Msg{keyNextTile, tuitest.KeyDown, tuitest.KeyEnter}, wantOverlays: []string{taskdetail.Name}, wantFocusedTileIndex: 1, wantActiveTab: "Tab", golden: "screen_task_overlay"},
		{name: "overlay takes the root keys", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, keyTabTwo, keyNextTile, tuitest.KeyDown, tuitest.KeyEnter}, wantOverlays: []string{taskdetail.Name}, wantActiveTab: "Tab"},
		{name: "help opens over the overlay", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, keyHelp}, wantOverlays: []string{taskdetail.Name, helpOverlayTitle}, wantActiveTab: "Tab", golden: "screen_help_over_task_overlay"},
		{name: "help takes the keys over the overlay", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, keyHelp, keyTabTwo, tuitest.KeyDown, tuitest.KeyEnter}, wantOverlays: []string{taskdetail.Name, helpOverlayTitle}, wantActiveTab: "Tab"},
		{name: "help key closes the help over the overlay", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, keyHelp, keyHelp}, wantOverlays: []string{taskdetail.Name}, wantActiveTab: "Tab"},
		{name: "esc closes the help over the overlay", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, keyHelp, tuitest.KeyEscape}, wantOverlays: []string{taskdetail.Name}, wantActiveTab: "Tab"},
		{name: "esc closes the help then the overlay", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, keyHelp, tuitest.KeyEscape, tuitest.KeyEscape}, wantActiveTab: "Tab"},
		{name: "esc with nothing open does nothing", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, keyHelp, tuitest.KeyEscape, tuitest.KeyEscape, tuitest.KeyEscape}, wantActiveTab: "Tab"},
		{name: "q closes the help then the overlay", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, keyHelp, keyQuit, keyQuit}, wantActiveTab: "Tab"},
		{name: "q closes the overlay and keeps focus", config: twoLists, keys: []tea.Msg{keyNextTile, tuitest.KeyEnter, keyQuit}, wantFocusedTileIndex: 1, wantActiveTab: "Tab", golden: "screen_task_overlay_closed"},
		{name: "esc closes the overlay and keeps focus", config: twoLists, keys: []tea.Msg{keyNextTile, tuitest.KeyEnter, tuitest.KeyEscape}, wantFocusedTileIndex: 1, wantActiveTab: "Tab", golden: "screen_task_overlay_closed"},
		{name: "keys work again after the overlay closes", config: twoLists, keys: []tea.Msg{tuitest.KeyEnter, tuitest.KeyEscape, keyTabTwo}, wantActiveTab: "Second"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, buildTestTabs(t, testCase.config), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
			startFeeds(model)
			for _, msg := range testCase.keys {
				_, cmd := model.Update(msg)
				if isQuit(cmd) {
					t.Fatalf("expected no quit from %v", msg)
				}
				runCommand(model, cmd)
			}
			if got := overlayTitles(model); !slices.Equal(got, testCase.wantOverlays) {
				t.Errorf("expected overlays %q, got %q", testCase.wantOverlays, got)
			}
			if got := model.ActiveTab().Title; got != testCase.wantActiveTab {
				t.Errorf("expected tab %q active, got %q", testCase.wantActiveTab, got)
			}
			if got := model.ActiveTab().focusedTileIndex; got != testCase.wantFocusedTileIndex {
				t.Errorf("expected tile %d focused, got %d", testCase.wantFocusedTileIndex, got)
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, model.View().Content)
			}
		})
	}
}

func TestModel_OpensRunLog(t *testing.T) {
	cases := []struct {
		name         string
		tasks        []task.ListedTask
		keys         []tea.Msg
		wantOverlays []string
		wantFilter   string
		golden       string
	}{
		{name: "L opens the runlog of the cursor task", tasks: tuitest.ListedTasks(), keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyRunLog}, wantOverlays: []string{"task-runlog [5b20d7aa] Write the backoff"}, golden: "screen_runlog_overlay"},
		{name: "L with no task under the cursor does nothing", keys: []tea.Msg{tuitest.KeyRunLog}},
		{name: "L goes into the open filter", tasks: tuitest.ListedTasks(), keys: []tea.Msg{tuitest.KeyFilter, tuitest.KeyRunLog}, wantFilter: "/L"},
		{name: "esc closes the runlog", tasks: tuitest.ListedTasks(), keys: []tea.Msg{tuitest.KeyRunLog, tuitest.KeyEscape}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModelWithSource(t, &fakeTaskSource{tasks: testCase.tasks}, testTabs(t, "Tasks"), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
			startFeeds(model)
			pressKeys(model, testCase.keys)
			if got := overlayTitles(model); !slices.Equal(got, testCase.wantOverlays) {
				t.Errorf("expected overlays %q, got %q", testCase.wantOverlays, got)
			}
			screen := model.View().Content
			if testCase.wantFilter != "" && !strings.Contains(ansi.Strip(screen), testCase.wantFilter) {
				t.Errorf("expected the filter %q on the screen, got:\n%s", testCase.wantFilter, ansi.Strip(screen))
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, screen)
			}
		})
	}
}

// runLogTitle returns the title of the runlog of a test task once it read
// the run.
func runLogTitle(listed task.ListedTask) string {
	return fmt.Sprintf("%s [%s] %s", taskrunlog.Name, task.ShortID(listed.Task.ID), listed.Task.Title)
}

func TestModel_OpensRunLogFromBoardAndDetail(t *testing.T) {
	keyNextColumn := tea.KeyPressMsg{Code: 'l', Text: "l"}
	board := oneTabConfig(NodeConfig{View: taskboard.Name})
	listAndDetail := oneTabConfig(NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), {View: taskdetail.Name}}})
	list := oneTabConfig(taskListNode())
	todoTask := tuitest.ListedTasks()[2]
	secondTask := tuitest.ListedTasks()[1]
	cases := []struct {
		name         string
		config       Config
		keys         []tea.Msg
		wantOverlays []string
	}{
		{name: "L on the board opens the runlog of the cursor task", config: board, keys: []tea.Msg{keyNextColumn, tuitest.KeyRunLog}, wantOverlays: []string{runLogTitle(todoTask)}},
		{name: "L on an empty column of the board does nothing", config: board, keys: []tea.Msg{keyNextColumn, keyNextColumn, keyNextColumn, keyNextColumn, tuitest.KeyRunLog}},
		{name: "L on the detail opens the runlog of the shown task", config: listAndDetail, keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyEnter, tuitest.KeyRunLog}, wantOverlays: []string{runLogTitle(secondTask)}},
		{name: "L on the detail with no task does nothing", config: listAndDetail, keys: []tea.Msg{keyNextTile, tuitest.KeyRunLog}},
		{name: "L on a detail overlay stacks the runlog on it", config: list, keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyEnter, tuitest.KeyRunLog}, wantOverlays: []string{taskdetail.Name, runLogTitle(secondTask)}},
		{name: "esc goes back to the detail overlay", config: list, keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyEnter, tuitest.KeyRunLog, tuitest.KeyEscape}, wantOverlays: []string{taskdetail.Name}},
		{name: "esc twice goes back to the tab", config: list, keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyEnter, tuitest.KeyRunLog, tuitest.KeyEscape, tuitest.KeyEscape}},
		{name: "L in the runlog does nothing", config: list, keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyRunLog, tuitest.KeyRunLog}, wantOverlays: []string{runLogTitle(secondTask)}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, buildTestTabs(t, testCase.config), nil)
			update(model, tea.WindowSizeMsg{Width: 120, Height: 16})
			startFeeds(model)
			pressKeys(model, testCase.keys)
			if got := overlayTitles(model); !slices.Equal(got, testCase.wantOverlays) {
				t.Errorf("expected overlays %q, got %q", testCase.wantOverlays, got)
			}
		})
	}
}

func TestModel_RunLogTileFollowsSelection(t *testing.T) {
	secondTask := tuitest.ListedTasks()[1]
	thirdTask := tuitest.ListedTasks()[2]
	cases := []struct {
		name      string
		keys      []tea.Msg
		wantTitle string
		wantFeeds []views.Feed
		golden    string
	}{
		{name: "nothing selected at start", wantTitle: taskrunlog.Name, golden: "screen_runlog_tile_nothing_selected"},
		{name: "a selection shows the run of the task", keys: []tea.Msg{tuitest.KeyDown}, wantTitle: runLogTitle(secondTask), wantFeeds: []views.Feed{{Name: views.RunLogFeed, TaskID: secondTask.Task.ID}}},
		{name: "another selection switches to the task and its feed", keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyDown}, wantTitle: runLogTitle(thirdTask), wantFeeds: []views.Feed{{Name: views.RunLogFeed, TaskID: thirdTask.Task.ID}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := oneTabConfig(NodeConfig{Arrangement: Stacked, Children: []NodeConfig{taskListNode(), {View: taskrunlog.Name}}})
			model := newTestModel(t, buildTestTabs(t, config), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 16})
			startFeeds(model)
			pressKeys(model, testCase.keys)
			runLogTile := model.ActiveTab().tiles[1]
			if got := runLogTile.Title(); got != testCase.wantTitle {
				t.Errorf("expected title %q, got %q", testCase.wantTitle, got)
			}
			if got := viewFeeds(runLogTile.view); !slices.Equal(got, testCase.wantFeeds) {
				t.Errorf("expected feeds %v, got %v", testCase.wantFeeds, got)
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, model.View().Content)
			}
		})
	}
}

// recordingView is a View that records the keys it gets. Enter opens the
// first test task.
type recordingView struct {
	sizeView
	isTakingAllKeys bool
	keys            []string
}

func (view *recordingView) TakesAllKeys() bool { return view.isTakingAllKeys }

func (view *recordingView) Update(msg tea.Msg) tea.Cmd {
	keyPress, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	view.keys = append(view.keys, keyPress.String())
	if keyPress.Code != tea.KeyEnter {
		return nil
	}
	openedID := tuitest.ListedTasks()[0].Task.ID
	return func() tea.Msg { return views.OpenTaskMsg{TaskID: openedID} }
}

// recordingViewName is the name of the recording View in the tests.
const recordingViewName = "recorder"

// pushRecordingView opens a recording View in an overlay of a model with the
// test tasks loaded.
func pushRecordingView(t *testing.T, isTakingAllKeys bool) (*Model, *recordingView) {
	t.Helper()
	model := newTestModel(t, testTabs(t, "Tasks"), nil)
	update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
	startFeeds(model)
	view := &recordingView{sizeView: sizeView{name: recordingViewName}, isTakingAllKeys: isTakingAllKeys}
	model.push(newViewOverlay(view, tuitest.Styles()))
	return model, view
}

func TestModel_OverlayViewThatTakesAllKeys(t *testing.T) {
	cases := []struct {
		name     string
		keys     []tea.Msg
		wantKeys []string
		wantQuit bool
	}{
		{name: "esc and the help key go to the view", keys: []tea.Msg{tuitest.KeyEscape, keyHelp}, wantKeys: []string{"esc", "?"}},
		{name: "root keys go to the view", keys: []tea.Msg{keyQuit, keyTabTwo}, wantKeys: []string{"q", "2"}},
		{name: "ctrl+c quits", keys: []tea.Msg{tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}}, wantQuit: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model, view := pushRecordingView(t, true)
			didQuit := false
			for _, msg := range testCase.keys {
				_, cmd := model.Update(msg)
				if isQuit(cmd) {
					didQuit = true
				}
			}
			if didQuit != testCase.wantQuit {
				t.Errorf("expected quit %t, got %t", testCase.wantQuit, didQuit)
			}
			if !slices.Equal(view.keys, testCase.wantKeys) {
				t.Errorf("expected the view to get %q, got %q", testCase.wantKeys, view.keys)
			}
			if got, want := overlayTitles(model), []string{recordingViewName}; !slices.Equal(got, want) {
				t.Errorf("expected overlays %q, got %q", want, got)
			}
		})
	}
}

func TestModel_OpensFromOverlay(t *testing.T) {
	model, _ := pushRecordingView(t, false)
	pressKeys(model, []tea.Msg{tuitest.KeyEnter})
	if got, want := overlayTitles(model), []string{recordingViewName, taskdetail.Name}; !slices.Equal(got, want) {
		t.Errorf("expected overlays %q, got %q", want, got)
	}
}

func TestModel_OverlayShowsLoadedTask(t *testing.T) {
	model := newTestModel(t, testTabs(t, "Tasks"), nil)
	update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
	startFeeds(model)
	pressKeys(model, []tea.Msg{tuitest.KeyEnter})
	top, ok := model.topOverlay()
	if !ok {
		t.Fatal("expected enter to open an overlay")
	}
	box, _, _ := top.Render()
	if want := task.ShortID(tuitest.ListedTasks()[0].Task.ID); !strings.Contains(ansi.Strip(box), want) {
		t.Errorf("expected the overlay to show task %q, got\n%s", want, ansi.Strip(box))
	}
}

// unknownOpenMsg is an open message no View shows.
type unknownOpenMsg struct {
	views.OpenMsg
}

func TestModel_OpenNoViewShows(t *testing.T) {
	cases := []struct {
		name string
		msg  func(model *Model) tea.Msg
	}{
		{name: "from a tab", msg: func(model *Model) tea.Msg { return tabMsg{tab: model.ActiveTab(), msg: unknownOpenMsg{}} }},
		{name: "from an overlay", msg: func(model *Model) tea.Msg { return overlayMsg{msg: unknownOpenMsg{}} }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, testTabs(t, "Tasks"), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
			startFeeds(model)
			before := model.View().Content
			_, cmd := model.Update(testCase.msg(model))
			runCommand(model, cmd)
			if got := overlayTitles(model); len(got) != 0 {
				t.Errorf("expected no overlay, got %q", got)
			}
			if after := model.View().Content; after != before {
				t.Errorf("expected the screen to stay the same, got:\n%s", after)
			}
		})
	}
}

// longDescriptionTasks returns the test tasks with a description on the first
// one that is too long for the task overlay.
func longDescriptionTasks() []task.ListedTask {
	tasks := tuitest.ListedTasks()
	tasks[0].Task.Description = strings.Repeat("Retry the upload.\n\n", 20)
	return tasks
}

func TestModel_OverlayMouse(t *testing.T) {
	cases := []struct {
		name          string
		msgs          []tea.Msg
		wantStatus    string
		wantActiveTab string
	}{
		{name: "the wheel scrolls the overlay", msgs: []tea.Msg{wheelDown(50, 5)}, wantStatus: "3f9a1c2e 3%", wantActiveTab: "Tab"},
		{name: "the wheel outside the overlay does nothing", msgs: []tea.Msg{wheelDown(0, 5)}, wantStatus: "3f9a1c2e 0%", wantActiveTab: "Tab"},
		{name: "the wheel on the border of the overlay does nothing", msgs: []tea.Msg{wheelDown(2, 5)}, wantStatus: "3f9a1c2e 0%", wantActiveTab: "Tab"},
		{name: "a click outside the overlay does nothing", msgs: []tea.Msg{leftClick(99, 5), leftClick(12, 0)}, wantStatus: "3f9a1c2e 0%", wantActiveTab: "Tab"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := oneTabConfig(NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), taskListNode()}})
			config.Tabs = append(config.Tabs, TabConfig{Title: "Second", Layout: taskListNode()})
			source := &fakeTaskSource{tasks: longDescriptionTasks()}
			model := newTestModelWithSource(t, source, buildTestTabs(t, config), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
			startFeeds(model)
			pressKeys(model, slices.Concat([]tea.Msg{tuitest.KeyEnter}, testCase.msgs))
			top, ok := model.topOverlay()
			if !ok {
				t.Fatal("expected the task overlay to stay open")
			}
			overlay, ok := top.(*viewOverlay)
			if !ok {
				t.Fatalf("expected a view overlay on top, got %T", top)
			}
			if got := overlay.tile.Status(); got != testCase.wantStatus {
				t.Errorf("expected overlay status %q, got %q", testCase.wantStatus, got)
			}
			if got := model.ActiveTab().Title; got != testCase.wantActiveTab {
				t.Errorf("expected tab %q active, got %q", testCase.wantActiveTab, got)
			}
			if got := model.ActiveTab().focusedTileIndex; got != 0 {
				t.Errorf("expected the first tile to keep focus, got tile %d", got)
			}
		})
	}
}

// twoDetailsConfig describes a Tab with a task list and two task details.
func twoDetailsConfig() Config {
	return oneTabConfig(NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{
		taskListNode(),
		{View: taskdetail.Name},
		{View: taskdetail.Name},
	}})
}

func TestModel_TwoTaskDetails(t *testing.T) {
	secondID := tuitest.ListedTasks()[1].Task.ID
	cases := []struct {
		name                 string
		keys                 []tea.Msg
		wantFocusedTileIndex int
	}{
		{name: "a selection shows the task in both", keys: []tea.Msg{tuitest.KeyDown}, wantFocusedTileIndex: 0},
		{name: "enter shows the task in both and focuses the first", keys: []tea.Msg{tuitest.KeyDown, tuitest.KeyEnter}, wantFocusedTileIndex: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, buildTestTabs(t, twoDetailsConfig()), nil)
			update(model, tea.WindowSizeMsg{Width: 150, Height: 24})
			startFeeds(model)
			pressKeys(model, testCase.keys)
			for _, tileIndex := range []int{1, 2} {
				if got := model.ActiveTab().tiles[tileIndex].view.Status(); got != task.ShortID(secondID) {
					t.Errorf("expected tile %d to show task %q, got status %q", tileIndex, secondID, got)
				}
			}
			if got := model.ActiveTab().focusedTileIndex; got != testCase.wantFocusedTileIndex {
				t.Errorf("expected tile %d focused, got %d", testCase.wantFocusedTileIndex, got)
			}
			if got := overlayTitles(model); len(got) != 0 {
				t.Errorf("expected no overlay, got %q", got)
			}
		})
	}
}

// stateProbe asks a running program for the state of its screen.
type stateProbe struct {
	reply chan<- probedState
}

type probedState struct {
	activeTab  string
	isHelpOpen bool
}

// probedModel answers a state probe from the goroutine of the program.
type probedModel struct {
	*Model
}

func (probed probedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if probe, ok := msg.(stateProbe); ok {
		isHelpOpen := slices.Equal(overlayTitles(probed.Model), []string{helpOverlayTitle})
		probe.reply <- probedState{activeTab: probed.ActiveTab().Title, isHelpOpen: isHelpOpen}
		return probed, nil
	}
	_, cmd := probed.Model.Update(msg)
	return probed, cmd
}

// probeState returns the state of the screen once the program handled every
// message sent before. It fails the test when the program does not answer,
// like after it quit.
func probeState(t *testing.T, program *tea.Program) probedState {
	t.Helper()
	reply := make(chan probedState, 1)
	program.Send(stateProbe{reply: reply})
	select {
	case state := <-reply:
		return state
	case <-time.After(waitTimeout):
		t.Fatalf("the program did not answer within %s, it may have quit", waitTimeout)
		return probedState{}
	}
}

func TestModel_HelpOverlayTakesKeysInProgram(t *testing.T) {
	sources := feeds.Sources{Tasks: &fakeTaskSource{tasks: tuitest.ListedTasks()}, Drudgers: &fakeDrudgerSource{}}
	program := startProgram(t, probedModel{newTestModelWithSources(t, sources, testTabs(t, "Tasks", "Board"), intervalsOf(longInterval), nil)})

	program.Send(keyHelp)
	if state := probeState(t, program); !state.isHelpOpen {
		t.Fatal("expected the help key to open the help")
	}

	program.Send(keyTabTwo)
	state := probeState(t, program)
	if !state.isHelpOpen {
		t.Error("expected the help to stay open")
	}
	if state.activeTab != "Tasks" {
		t.Errorf("expected tab %q to stay active, got %q", "Tasks", state.activeTab)
	}

	program.Send(tuitest.KeyEscape)
	if state := probeState(t, program); state.isHelpOpen {
		t.Error("expected esc to close the help")
	}
}

// isQuit reports whether a command quits the program.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// tallLeftConfig describes a Tab with one tall Tile on the left and three
// stacked on the right.
func tallLeftConfig(title string) TabConfig {
	return TabConfig{Title: title, Layout: NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{
		{View: tasklist.Name, Title: "Left"},
		{Arrangement: Stacked, Children: []NodeConfig{
			{View: tasklist.Name, Title: "Top"},
			{View: tasklist.Name, Title: "Middle"},
			{View: tasklist.Name, Title: "Bottom"},
		}},
	}}}
}

func TestModel_MovesFocusAndZooms(t *testing.T) {
	cases := []struct {
		name            string
		keys            []tea.KeyPressMsg
		wantFocusedTile string
		wantZoomed      bool
		golden          string
	}{
		{name: "first tile at start", wantFocusedTile: "Left", golden: "screen_focus_first_tile"},
		{name: "right to the middle of three", keys: []tea.KeyPressMsg{keyFocusRight}, wantFocusedTile: "Middle", golden: "screen_focus_moved"},
		{name: "right then down", keys: []tea.KeyPressMsg{keyFocusRight, keyFocusDown}, wantFocusedTile: "Bottom"},
		{name: "right then up", keys: []tea.KeyPressMsg{keyFocusRight, keyFocusUp}, wantFocusedTile: "Top"},
		{name: "right then left", keys: []tea.KeyPressMsg{keyFocusRight, keyFocusLeft}, wantFocusedTile: "Left"},
		{name: "arrows", keys: []tea.KeyPressMsg{keyArrowRight, keyArrowUp, keyArrowDown, keyArrowDown, keyArrowLeft}, wantFocusedTile: "Left"},
		{name: "arrow right then down", keys: []tea.KeyPressMsg{keyArrowRight, keyArrowDown}, wantFocusedTile: "Bottom"},
		{name: "left with no tile there", keys: []tea.KeyPressMsg{keyFocusLeft}, wantFocusedTile: "Left"},
		{name: "next tile in tree order", keys: []tea.KeyPressMsg{keyNextTile, keyNextTile}, wantFocusedTile: "Middle"},
		{name: "next tile wraps to the first", keys: []tea.KeyPressMsg{keyNextTile, keyNextTile, keyNextTile, keyNextTile}, wantFocusedTile: "Left"},
		{name: "previous tile wraps to the last", keys: []tea.KeyPressMsg{keyPreviousTile}, wantFocusedTile: "Bottom"},
		{name: "zoom", keys: []tea.KeyPressMsg{keyFocusRight, keyZoom}, wantFocusedTile: "Middle", wantZoomed: true, golden: "screen_zoomed"},
		{name: "zoom again restores the tiles", keys: []tea.KeyPressMsg{keyFocusRight, keyZoom, keyZoom}, wantFocusedTile: "Middle", golden: "screen_unzoomed"},
		{name: "focus moves the zoom", keys: []tea.KeyPressMsg{keyZoom, keyNextTile}, wantFocusedTile: "Top", wantZoomed: true, golden: "screen_zoom_moved"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, buildTestTabs(t, Config{Tabs: []TabConfig{tallLeftConfig("Tab")}}), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 24})
			startFeeds(model)
			for _, keyPress := range testCase.keys {
				if _, cmd := model.Update(keyPress); cmd != nil {
					t.Errorf("expected no command from %q", keyPress.String())
				}
			}
			activeTab := model.ActiveTab()
			if got := activeTab.FocusedTile().Title(); got != testCase.wantFocusedTile {
				t.Errorf("expected focus on tile %q, got %q", testCase.wantFocusedTile, got)
			}
			if got := activeTab.IsZoomed(); got != testCase.wantZoomed {
				t.Errorf("expected zoomed %v, got %v", testCase.wantZoomed, got)
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, model.View().Content)
			}
		})
	}
}

func leftClick(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func wheelDown(x, y int) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown}
}

func wheelUp(x, y int) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelUp}
}

// Cells of a screen of 100 by 24 with a tall left Tile and three stacked on
// the right, under the top bar of the Tabs First and Second. The Top Tile has
// room for four of the five tasks.
var (
	clickSecondTabLabel = leftClick(12, 0)
	clickPastTabLabels  = leftClick(90, 0)
	clickTopBorder      = leftClick(60, 1)
	clickTopFirstLine   = leftClick(60, 3)
	clickTopSecondLine  = leftClick(60, 4)
	wheelDownTop        = wheelDown(60, 4)
	wheelUpTop          = wheelUp(60, 4)
	rightClickTop       = tea.MouseClickMsg{X: 60, Y: 4, Button: tea.MouseRight}
	releaseTop          = tea.MouseReleaseMsg{X: 60, Y: 4, Button: tea.MouseLeft}
	clickMiddleHeader   = leftClick(60, 9)
	clickMiddleThirdRow = leftClick(60, 12)
	clickLeftSecondRow  = leftClick(10, 4)
	clickBottomBar      = leftClick(60, 23)
)

func TestModel_Mouse(t *testing.T) {
	cases := []struct {
		name            string
		msgs            []tea.Msg
		wantActiveTab   string
		wantFocusedTile string
		wantStatuses    []string
		golden          string
	}{
		{name: "a click on a tab label switches tabs", msgs: []tea.Msg{clickSecondTabLabel}, wantActiveTab: "Second", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a click past the tab labels does nothing", msgs: []tea.Msg{clickPastTabLabels}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a click on a border focuses the tile", msgs: []tea.Msg{clickTopBorder}, wantActiveTab: "First", wantFocusedTile: "Top", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a click on a header focuses the tile", msgs: []tea.Msg{clickMiddleHeader}, wantActiveTab: "First", wantFocusedTile: "Middle", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a click on a row moves the cursor there", msgs: []tea.Msg{clickMiddleThirdRow}, wantActiveTab: "First", wantFocusedTile: "Middle", wantStatuses: []string{"1/5", "1/5", "3/5", "1/5"}, golden: "screen_mouse_clicked_row"},
		{name: "a click on a row of the focused tile", msgs: []tea.Msg{clickLeftSecondRow}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"2/5", "1/5", "1/5", "1/5"}},
		{name: "a click on the bottom bar does nothing", msgs: []tea.Msg{clickBottomBar}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a right click does nothing", msgs: []tea.Msg{rightClickTop}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a mouse release does nothing", msgs: []tea.Msg{releaseTop}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a click while the help is open does nothing", msgs: []tea.Msg{keyHelp, clickMiddleThirdRow, clickSecondTabLabel}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "the wheel scrolls an unfocused tile", msgs: []tea.Msg{wheelDownTop}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "2/5", "1/5", "1/5"}, golden: "screen_mouse_wheel_scrolled"},
		{name: "a click on a scrolled tile counts from the first row in sight", msgs: []tea.Msg{wheelDownTop, clickTopSecondLine}, wantActiveTab: "First", wantFocusedTile: "Top", wantStatuses: []string{"1/5", "3/5", "1/5", "1/5"}},
		{name: "the wheel stops at the last row", msgs: []tea.Msg{wheelDownTop, wheelDownTop, wheelDownTop, clickTopFirstLine}, wantActiveTab: "First", wantFocusedTile: "Top", wantStatuses: []string{"1/5", "2/5", "1/5", "1/5"}},
		{name: "the wheel scrolls back up", msgs: []tea.Msg{wheelDownTop, wheelUpTop, clickTopFirstLine}, wantActiveTab: "First", wantFocusedTile: "Top", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a click at the cell of another tile goes to the zoomed tile", msgs: []tea.Msg{keyZoom, clickTopSecondLine}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"2/5", "1/5", "1/5", "1/5"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := Config{Tabs: []TabConfig{tallLeftConfig("First"), tallLeftConfig("Second")}}
			model := newTestModel(t, buildTestTabs(t, config), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 24})
			startFeeds(model)
			for _, msg := range testCase.msgs {
				model.Update(msg)
			}
			activeTab := model.ActiveTab()
			if got := activeTab.Title; got != testCase.wantActiveTab {
				t.Errorf("expected tab %q active, got %q", testCase.wantActiveTab, got)
			}
			if got := activeTab.FocusedTile().Title(); got != testCase.wantFocusedTile {
				t.Errorf("expected focus on tile %q, got %q", testCase.wantFocusedTile, got)
			}
			if got := tileStatuses(activeTab); !slices.Equal(got, testCase.wantStatuses) {
				t.Errorf("expected statuses %q, got %q", testCase.wantStatuses, got)
			}
			if testCase.golden != "" {
				tuitest.AssertGolden(t, testCase.golden, model.View().Content)
			}
		})
	}
}

func TestModel_ClickOnScrolledTabBar(t *testing.T) {
	cases := []struct {
		name          string
		x             int
		wantActiveTab string
	}{
		{name: "the first label shown is the first tab not hidden", x: 0, wantActiveTab: "Drudgers"},
		{name: "the last cell of a label", x: 11, wantActiveTab: "Drudgers"},
		{name: "the first cell of the next label", x: 12, wantActiveTab: "Backlog"},
		{name: "past the labels", x: 25, wantActiveTab: "Backlog"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, testTabs(t, "Tasks", "Board", "Drudgers", "Backlog"), nil)
			update(model, tea.WindowSizeMsg{Width: 30, Height: 8})
			model.Update(keyPreviousTab)
			model.Update(leftClick(testCase.x, 0))
			if got := model.ActiveTab().Title; got != testCase.wantActiveTab {
				t.Errorf("expected tab %q active, got %q", testCase.wantActiveTab, got)
			}
		})
	}
}

func TestModel_TabsKeepFocusedTile(t *testing.T) {
	config := Config{Tabs: []TabConfig{tallLeftConfig("First"), tallLeftConfig("Second")}}
	model := newTestModel(t, buildTestTabs(t, config), nil)
	update(model, tea.WindowSizeMsg{Width: 100, Height: 24})
	keys := []tea.KeyPressMsg{keyFocusRight, keyFocusDown, keyNextTab, keyNextTile, keyPreviousTab}
	for _, keyPress := range keys {
		model.Update(keyPress)
	}

	cases := []struct {
		tabKey          tea.KeyPressMsg
		wantFocusedTile string
	}{
		{tabKey: keyTabOne, wantFocusedTile: "Bottom"},
		{tabKey: keyTabTwo, wantFocusedTile: "Top"},
		{tabKey: keyTabOne, wantFocusedTile: "Bottom"},
	}
	for _, testCase := range cases {
		model.Update(testCase.tabKey)
		if got := model.ActiveTab().FocusedTile().Title(); got != testCase.wantFocusedTile {
			t.Errorf("expected tab %q to keep focus on tile %q, got %q", model.ActiveTab().Title, testCase.wantFocusedTile, got)
		}
	}
}

func TestModel_TickWhileRunGoesStartsNoSecondRun(t *testing.T) {
	source := &fakeTaskSource{tasks: tuitest.ListedTasks(), startedCalls: make(chan int, 100), release: make(chan struct{})}
	intervals := intervalsOf(longInterval)
	intervals[views.TasksFeed] = shortInterval
	sources := feeds.Sources{Tasks: source, Drudgers: &fakeDrudgerSource{}}
	startProgram(t, newTestModelWithSources(t, sources, testTabs(t, "Tasks"), intervals, nil))

	waitForCall(t, source.startedCalls, 1)
	select {
	case number := <-source.startedCalls:
		t.Fatalf("call %d started while call 1 still ran", number)
	case <-time.After(4 * shortInterval):
	}

	close(source.release)
	waitForCall(t, source.startedCalls, 2)
}

func TestModel_FeedsFollowTheirIntervals(t *testing.T) {
	cases := []struct {
		name      string
		shortFeed views.FeedName
		// wantTasks, wantReads, wantChecks and wantSessionChecks are the calls
		// each feed starts. A count above 1 comes from the ticks of its
		// interval.
		wantTasks         int
		wantReads         int
		wantChecks        int
		wantSessionChecks int
	}{
		{name: "tasks follow their interval", shortFeed: views.TasksFeed, wantTasks: 3, wantReads: 1, wantChecks: 1, wantSessionChecks: 1},
		{name: "reads follow their interval", shortFeed: views.DrudgersFeed, wantTasks: 1, wantReads: 3, wantChecks: 1, wantSessionChecks: 1},
		{name: "checks follow their interval", shortFeed: views.DrudgerCheckFeed, wantTasks: 1, wantReads: 1, wantChecks: 3, wantSessionChecks: 1},
		{name: "Session checks follow their interval", shortFeed: views.SessionsFeed, wantTasks: 1, wantReads: 1, wantChecks: 1, wantSessionChecks: 3},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tasks := &fakeTaskSource{tasks: tuitest.ListedTasks(), startedCalls: make(chan int, 100)}
			drudgers := &fakeDrudgerSource{startedReads: make(chan int, 100), startedChecks: make(chan int, 100), startedSessionChecks: make(chan int, 100)}
			intervals := intervalsOf(longInterval)
			intervals[testCase.shortFeed] = shortInterval
			config := oneTabConfig(NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), {View: drudgerlist.Name}}})
			sources := feeds.Sources{Tasks: tasks, Drudgers: drudgers}
			startProgram(t, newTestModelWithSources(t, sources, buildTestTabs(t, config), intervals, nil))

			waitForCall(t, tasks.startedCalls, 1)
			waitForCall(t, drudgers.startedReads, 1)
			waitForCall(t, drudgers.startedChecks, 1)
			waitForCall(t, drudgers.startedSessionChecks, 1)
			assertCalls(t, "task lists", tasks.startedCalls, &tasks.callCount, testCase.wantTasks)
			assertCalls(t, "reads", drudgers.startedReads, &drudgers.readCount, testCase.wantReads)
			assertCalls(t, "checks", drudgers.startedChecks, &drudgers.checkCount, testCase.wantChecks)
			assertCalls(t, "Session checks", drudgers.startedSessionChecks, &drudgers.sessionCheckCount, testCase.wantSessionChecks)
		})
	}
}

// assertCalls waits until a source starts the call numbered wantCount after
// its first call. A want of 1 expects no call after the first.
func assertCalls(t *testing.T, name string, startedCalls chan int, callCount *atomic.Int32, wantCount int) {
	t.Helper()
	if wantCount > 1 {
		waitForCall(t, startedCalls, wantCount)
		return
	}
	if got := int(callCount.Load()); got != 1 {
		t.Errorf("expected 1 of the %s, got %d", name, got)
	}
}

// Tabs of the feed tests: one with a task list and one with a Drudger list.
var (
	taskListTab    = TabConfig{Title: "Tasks", Layout: NodeConfig{View: tasklist.Name}}
	drudgerListTab = TabConfig{Title: "Drudgers", Layout: NodeConfig{View: drudgerlist.Name}}
	taskBoardTab   = TabConfig{Title: "Board", Layout: NodeConfig{View: taskboard.Name}}
)

// openTask returns a message that opens the first test task from the active
// Tab.
func openTask(model *Model) tea.Msg {
	return tabMsg{tab: model.ActiveTab(), msg: views.OpenTaskMsg{TaskID: tuitest.ListedTasks()[0].Task.ID}}
}

func TestModel_RunsFeedsOfVisibleViews(t *testing.T) {
	cases := []struct {
		name string
		tabs []TabConfig
		// msgs are sent after the feeds started.
		msgs              []func(model *Model) tea.Msg
		wantTasks         int
		wantReads         int
		wantChecks        int
		wantSessionChecks int
	}{
		{name: "a drudger list reads and checks the Drudgers and lists no tasks", tabs: []TabConfig{drudgerListTab}, wantReads: 1, wantChecks: 1},
		{name: "a task board lists the tasks and checks the Sessions", tabs: []TabConfig{taskBoardTab}, wantTasks: 1, wantSessionChecks: 1},
		{name: "an inactive tab runs no feed", tabs: []TabConfig{taskListTab, drudgerListTab}, wantTasks: 1, wantSessionChecks: 1},
		{
			name:              "switching tabs starts the feeds of the new tab",
			tabs:              []TabConfig{taskListTab, drudgerListTab},
			msgs:              []func(model *Model) tea.Msg{pressed(keyTabTwo)},
			wantTasks:         1,
			wantReads:         1,
			wantChecks:        1,
			wantSessionChecks: 1,
		},
		{
			name:              "switching back starts the feeds of the old tab again",
			tabs:              []TabConfig{taskListTab, drudgerListTab},
			msgs:              []func(model *Model) tea.Msg{pressed(keyTabTwo), pressed(keyTabOne)},
			wantTasks:         2,
			wantReads:         1,
			wantChecks:        1,
			wantSessionChecks: 2,
		},
		{
			name:              "switching between tabs with the same feeds starts nothing",
			tabs:              []TabConfig{taskListTab, taskBoardTab},
			msgs:              []func(model *Model) tea.Msg{pressed(keyTabTwo), pressed(keyTabOne)},
			wantTasks:         1,
			wantSessionChecks: 1,
		},
		{
			name:              "an overlay starts the feeds of its view",
			tabs:              []TabConfig{drudgerListTab},
			msgs:              []func(model *Model) tea.Msg{openTask},
			wantTasks:         1,
			wantReads:         1,
			wantChecks:        1,
			wantSessionChecks: 1,
		},
		{
			name:              "closing an overlay stops the feeds of its view",
			tabs:              []TabConfig{drudgerListTab},
			msgs:              []func(model *Model) tea.Msg{openTask, pressed(tuitest.KeyEscape), openTask},
			wantTasks:         2,
			wantReads:         1,
			wantChecks:        1,
			wantSessionChecks: 2,
		},
		{
			name:              "closing an overlay keeps the feeds the tab shows",
			tabs:              []TabConfig{taskListTab},
			msgs:              []func(model *Model) tea.Msg{openTask, pressed(tuitest.KeyEscape)},
			wantTasks:         1,
			wantSessionChecks: 1,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tasks := &fakeTaskSource{tasks: tuitest.ListedTasks()}
			drudgers := &fakeDrudgerSource{}
			sources := feeds.Sources{Tasks: tasks, Drudgers: drudgers}
			model := newTestModelWithSources(t, sources, buildTestTabs(t, Config{Tabs: testCase.tabs}), intervalsOf(instantInterval), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 12})
			startFeeds(model)
			for _, msg := range testCase.msgs {
				_, cmd := model.Update(msg(model))
				runCommand(model, cmd)
			}

			if got := int(tasks.callCount.Load()); got != testCase.wantTasks {
				t.Errorf("expected %d task lists, got %d", testCase.wantTasks, got)
			}
			if got := int(drudgers.readCount.Load()); got != testCase.wantReads {
				t.Errorf("expected %d reads of the Drudgers, got %d", testCase.wantReads, got)
			}
			if got := int(drudgers.checkCount.Load()); got != testCase.wantChecks {
				t.Errorf("expected %d checks of the Drudgers, got %d", testCase.wantChecks, got)
			}
			if got := int(drudgers.sessionCheckCount.Load()); got != testCase.wantSessionChecks {
				t.Errorf("expected %d checks of the Sessions, got %d", testCase.wantSessionChecks, got)
			}
		})
	}
}

// pressed returns a key press for the feed tests.
func pressed(keyPress tea.KeyPressMsg) func(model *Model) tea.Msg {
	return func(*Model) tea.Msg { return keyPress }
}

// subscriberView is a View that shows feeds and records the messages it gets.
type subscriberView struct {
	sizeView
	feeds    []views.Feed
	messages []tea.Msg
}

func (view *subscriberView) Feeds() []views.Feed { return view.feeds }

func (view *subscriberView) Update(msg tea.Msg) tea.Cmd {
	view.messages = append(view.messages, msg)
	return nil
}

// messageTypes returns the type of every message a View got, in order.
func messageTypes(messages []tea.Msg) []string {
	types := make([]string, 0, len(messages))
	for _, msg := range messages {
		types = append(types, fmt.Sprintf("%T", msg))
	}
	return types
}

func TestModel_DeliversToSubscribers(t *testing.T) {
	tasksFeed := []views.Feed{{Name: views.TasksFeed}}
	drudgersFeed := []views.Feed{{Name: views.DrudgersFeed}}
	tasksLoaded := fmt.Sprintf("%T", views.TasksLoadedMsg{})
	drudgersLoaded := fmt.Sprintf("%T", views.DrudgersLoadedMsg{})
	cases := []struct {
		name string
		view *subscriberView
		// isInActiveTab puts the View in the active Tab beside a task list and
		// a Drudger list. Otherwise it is in a second Tab.
		isInActiveTab bool
		wantMessages  []string
	}{
		{name: "a view of the tasks feed gets the tasks", view: &subscriberView{feeds: tasksFeed}, isInActiveTab: true, wantMessages: []string{tasksLoaded}},
		{name: "a view of the Drudgers feed gets the Drudgers", view: &subscriberView{feeds: drudgersFeed}, isInActiveTab: true, wantMessages: []string{drudgersLoaded}},
		{name: "a view of no feed gets nothing", view: &subscriberView{}, isInActiveTab: true, wantMessages: []string{}},
		{name: "a view in an inactive tab gets the data of a visible view", view: &subscriberView{feeds: tasksFeed}, wantMessages: []string{tasksLoaded}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			screenStyles := tuitest.Styles()
			subscriberTile := NewTile(testCase.view, "", screenStyles, 1, true)
			activeTiles := []Node{
				NewTile(tasklist.New(screenStyles, ""), "", screenStyles, 1, true),
				NewTile(drudgerlist.New(screenStyles), "", screenStyles, 1, true),
			}
			secondLayout := Node(NewTile(&sizeView{name: "other"}, "", screenStyles, 1, true))
			if testCase.isInActiveTab {
				activeTiles = append(activeTiles, subscriberTile)
			} else {
				secondLayout = subscriberTile
			}
			activeTab, err := NewTab("First", NewSplit(SideBySide, 1, activeTiles...))
			if err != nil {
				t.Fatal(err)
			}
			secondTab, err := NewTab("Second", secondLayout)
			if err != nil {
				t.Fatal(err)
			}
			model := newTestModel(t, []*Tab{activeTab, secondTab}, nil)
			startFeeds(model)

			if got := messageTypes(testCase.view.messages); !slices.Equal(got, testCase.wantMessages) {
				t.Errorf("expected the view to get %q, got %q", testCase.wantMessages, got)
			}
		})
	}
}

func TestModel_FailedRunShowsError(t *testing.T) {
	cases := []struct {
		name          string
		tabs          []TabConfig
		failFromCall  int
		checkErr      error
		keys          []tea.KeyPressMsg
		wantContent   []string
		wantNoContent []string
	}{
		{
			name:         "a failed list shows its error",
			tabs:         []TabConfig{taskListTab},
			failFromCall: 1,
			wantContent:  []string{"✗ disk on fire"},
		},
		{
			name:        "a failed check shows its error",
			tabs:        []TabConfig{drudgerListTab},
			checkErr:    errDiskOnFire,
			wantContent: []string{"✗ disk on fire"},
		},
		{
			name:         "a failed list keeps the last data on screen",
			tabs:         []TabConfig{taskListTab, drudgerListTab},
			failFromCall: 2,
			keys:         []tea.KeyPressMsg{keyTabTwo, keyTabOne},
			wantContent:  []string{"Add retry to uploader", "✗ disk on fire"},
		},
		{
			name:          "the error goes away when its feed stops",
			tabs:          []TabConfig{taskListTab, drudgerListTab},
			failFromCall:  1,
			keys:          []tea.KeyPressMsg{keyTabTwo},
			wantNoContent: []string{"disk on fire"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tasks := &fakeTaskSource{tasks: tuitest.ListedTasks(), failFromCall: testCase.failFromCall}
			sources := feeds.Sources{Tasks: tasks, Drudgers: &fakeDrudgerSource{checkErr: testCase.checkErr}}
			model := newTestModelWithSources(t, sources, buildTestTabs(t, Config{Tabs: testCase.tabs}), intervalsOf(instantInterval), nil)
			update(model, tea.WindowSizeMsg{Width: 140, Height: 12})
			startFeeds(model)
			for _, keyPress := range testCase.keys {
				_, cmd := model.Update(keyPress)
				runCommand(model, cmd)
			}

			content := model.View().Content
			for _, want := range testCase.wantContent {
				if !strings.Contains(content, want) {
					t.Errorf("expected the screen to show %q, got:\n%s", want, content)
				}
			}
			for _, unwanted := range testCase.wantNoContent {
				if strings.Contains(content, unwanted) {
					t.Errorf("expected the screen not to show %q, got:\n%s", unwanted, content)
				}
			}
		})
	}
}

func TestModel_BottomBar(t *testing.T) {
	cases := []struct {
		name         string
		width        int
		failFromCall int
		warnings     []string
		keys         []tea.KeyPressMsg
	}{
		{name: "bottom_bar_hint", width: 140, keys: []tea.KeyPressMsg{tuitest.KeyDown, tuitest.KeyDown}},
		{name: "bottom_bar_filter_hides_hint", width: 140, keys: []tea.KeyPressMsg{tuitest.KeyFilter}},
		{
			name:         "bottom_bar_errors_and_warnings",
			width:        240,
			failFromCall: 1,
			warnings:     []string{`theme.json: "red" is not a color for role error, using the theme's own`},
		},
		{name: "bottom_bar_overflow_cuts_errors", width: 60, failFromCall: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			source := &fakeTaskSource{tasks: tuitest.ListedTasks(), failFromCall: testCase.failFromCall}
			model := newTestModelWithSource(t, source, testTabs(t, "Tasks"), testCase.warnings)
			update(model, tea.WindowSizeMsg{Width: testCase.width, Height: 24})
			startFeeds(model)
			for _, keyPress := range testCase.keys {
				model.Update(keyPress)
			}
			lines := strings.Split(model.View().Content, "\n")
			tuitest.AssertGolden(t, testCase.name, lines[len(lines)-1])
		})
	}
}

func TestModel_TileStatuses(t *testing.T) {
	threeTiles := func(middleShowStatus *bool) Config {
		return oneTabConfig(NodeConfig{Arrangement: Stacked, Children: []NodeConfig{
			{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), {View: tasklist.Name, ShowStatus: middleShowStatus}}},
			taskListNode(),
		}})
	}
	cases := []struct {
		name   string
		config Config
		keys   []tea.KeyPressMsg
	}{
		{name: "screen_tile_statuses", config: threeTiles(nil), keys: []tea.KeyPressMsg{tuitest.KeyDown, tuitest.KeyDown}},
		{name: "screen_tile_status_hidden", config: threeTiles(new(false)), keys: []tea.KeyPressMsg{tuitest.KeyDown, tuitest.KeyDown}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, buildTestTabs(t, testCase.config), nil)
			update(model, tea.WindowSizeMsg{Width: 100, Height: 24})
			startFeeds(model)
			for _, keyPress := range testCase.keys {
				model.Update(keyPress)
			}
			tuitest.AssertGolden(t, testCase.name, model.View().Content)
		})
	}
}

// tileStatuses returns the status of every Tile of a Tab in tree order.
func tileStatuses(tab *Tab) []string {
	statuses := make([]string, 0, len(tab.tiles))
	for _, tile := range tab.tiles {
		statuses = append(statuses, tile.Status())
	}
	return statuses
}

func TestNewModel_NoTabs(t *testing.T) {
	sources := feeds.Sources{Tasks: &fakeTaskSource{}, Drudgers: &fakeDrudgerSource{}}
	_, err := NewModel(testProject, sources, newTestCommands(&fakeCommandService{}), tuitest.Styles(), nil, intervalsOf(instantInterval), true, nil)
	if err == nil {
		t.Fatal("expected an error for a screen with no tabs")
	}
}

func TestRun_OutsideProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	err := Run()
	if err == nil {
		t.Fatal("expected an error outside a project directory")
	}
	if !strings.Contains(err.Error(), "drg project init") {
		t.Errorf("expected the error to point at drg project init, got %q", err)
	}
}
