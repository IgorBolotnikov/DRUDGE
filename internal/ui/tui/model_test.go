package tui

import (
	"errors"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

var testProject = &project.Project{Slug: "uploader", Name: "Uploader"}

var errDiskOnFire = errors.New("disk on fire")

// testRefreshInterval is long enough that no tick comes during a test that
// refreshes with the refresh key.
const testRefreshInterval = time.Hour

// waitTimeout caps how long a test waits for a load of a running program.
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

// fakeDrudgerSource reads and checks its Drudgers. A read sends its number on
// startedReads and a check sends its number on startedChecks when they are
// set. A check waits for releaseChecks when it is set. A check fails with
// checkErr.
type fakeDrudgerSource struct {
	drudgers      []*drudger.Drudger
	checkErr      error
	startedReads  chan int
	startedChecks chan int
	releaseChecks chan struct{}
	readCount     atomic.Int32
	checkCount    atomic.Int32
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

var (
	keyRefresh = tea.KeyPressMsg{Code: 'r', Text: "r"}
	keyCheck   = tea.KeyPressMsg{Code: 'r', Text: "R", Mod: tea.ModShift}
	keyQuit    = tea.KeyPressMsg{Code: 'q', Text: "q"}
	keyHelp    = tea.KeyPressMsg{Code: '?', Text: "?"}
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
	return newTestModelWithSource(t, &fakeTaskSource{tasks: tuitest.ListedTasks()}, tabs, testRefreshInterval, warnings)
}

// newTestModelWithSource returns a Model of the test project that lists the
// tasks of source and has no Drudgers. Its Drudger check waits for the test
// refresh interval.
func newTestModelWithSource(t *testing.T, source TaskSource, tabs []*Tab, refreshInterval time.Duration, warnings []string) *Model {
	t.Helper()
	sources := Sources{Tasks: source, Drudgers: &fakeDrudgerSource{}}
	return newTestModelWithSources(t, sources, tabs, Intervals{Refresh: refreshInterval, DrudgerCheck: testRefreshInterval}, warnings)
}

func newTestModelWithSources(t *testing.T, sources Sources, tabs []*Tab, intervals Intervals, warnings []string) *Model {
	t.Helper()
	model, err := NewModel(testProject, sources, tuitest.Styles(), tabs, intervals, true, warnings)
	if err != nil {
		t.Fatal(err)
	}
	return model
}

// refreshNow presses the refresh key and runs the loads it starts.
func refreshNow(model *Model) {
	_, cmd := model.Update(keyRefresh)
	runCommand(model, cmd)
}

// runCommand runs a command and every command its messages lead to. It never
// gets a refresh tick, since a tick waits for the refresh interval.
func runCommand(model *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, batched := range batch {
			runCommand(model, batched)
		}
		return
	}
	_, next := model.Update(msg)
	runCommand(model, next)
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
			model.Update(tea.WindowSizeMsg{Width: testCase.width, Height: testCase.height})
			refreshNow(model)
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
		{name: "matching ignores case", keys: slices.Concat([]tea.Msg{tuitest.KeyFilter}, tuitest.Type("RETR")), wantActiveTab: "Tasks", wantStatuses: []string{"/RETR 1/2"}, golden: "screen_filtered"},
		{name: "a digit goes to the filter", keys: slices.Concat([]tea.Msg{tuitest.KeyFilter}, tuitest.Type("2")), wantActiveTab: "Tasks", wantStatuses: []string{"/2 0/0"}},
		{name: "root keys go to the filter", keys: []tea.Msg{tuitest.KeyFilter, keyQuit, keyNextTab, keyPreviousTab, keyRefresh, keyZoom}, wantActiveTab: "Tasks", wantStatuses: []string{"/q][rz 0/0"}},
		{name: "ctrl+c quits from the filter", keys: []tea.Msg{tuitest.KeyFilter, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}}, wantActiveTab: "Tasks", wantStatuses: []string{"/ 1/5"}, wantQuit: true},
		{name: "esc restores the full list", keys: slices.Concat([]tea.Msg{tuitest.KeyFilter}, tuitest.Type("RETR"), []tea.Msg{tuitest.KeyEscape}), wantActiveTab: "Tasks", wantStatuses: []string{"1/5"}},
		{name: "a digit switches tabs after esc", keys: []tea.Msg{tuitest.KeyFilter, tuitest.KeyEscape, keyTabTwo}, wantActiveTab: "Board", wantStatuses: []string{"1/5"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, testTabs(t, "Tasks", "Board"), nil)
			model.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
			refreshNow(model)
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
			if got := model.ActiveTab().Statuses(); !slices.Equal(got, testCase.wantStatuses) {
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
		wantHelpOpen  bool
		wantActiveTab string
		golden        string
	}{
		{name: "help opens", width: 100, height: 24, keys: []tea.Msg{keyHelp}, wantHelpOpen: true, wantActiveTab: "First", golden: "screen_help"},
		{name: "help in a terminal smaller than the box", width: 30, height: 8, keys: []tea.Msg{keyHelp}, wantHelpOpen: true, wantActiveTab: "First", golden: "screen_help_too_small"},
		{name: "help lists the keys of the focused view", width: 100, height: 24, keys: []tea.Msg{keyNextTile, keyHelp}, wantHelpOpen: true, wantActiveTab: "First"},
		{name: "esc closes help", width: 100, height: 24, keys: []tea.Msg{keyHelp, tuitest.KeyEscape}, wantActiveTab: "First"},
		{name: "help key closes help", width: 100, height: 24, keys: []tea.Msg{keyHelp, keyHelp}, wantActiveTab: "First"},
		{
			name:          "help takes the root and view keys",
			width:         100,
			height:        24,
			keys:          []tea.Msg{keyHelp, keyNextTab, keyPreviousTab, keyTabTwo, keyZoom, keyNextTile, tuitest.KeyDown, tuitest.KeyFilter},
			wantHelpOpen:  true,
			wantActiveTab: "First",
			golden:        "screen_help_takes_keys",
		},
		{name: "the help key goes to an open filter", width: 100, height: 24, keys: []tea.Msg{tuitest.KeyFilter, keyHelp}, wantActiveTab: "First"},
		{name: "keys work again after help closes", width: 100, height: 24, keys: []tea.Msg{keyHelp, tuitest.KeyEscape, keyTabTwo}, wantActiveTab: "Second"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := Config{Tabs: []TabConfig{
				oneTabConfig(NodeConfig{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), taskListNode()}}).Tabs[0],
				{Title: "Second", Layout: taskListNode()},
			}}
			config.Tabs[0].Title = "First"
			model := newTestModel(t, buildTestTabs(t, config), nil)
			model.Update(tea.WindowSizeMsg{Width: testCase.width, Height: testCase.height})
			refreshNow(model)
			for _, msg := range testCase.keys {
				_, cmd := model.Update(msg)
				if isQuit(cmd) {
					t.Fatalf("expected no quit from %v", msg)
				}
			}
			if got := model.helpOverlay.isOpen; got != testCase.wantHelpOpen {
				t.Errorf("expected help open %t, got %t", testCase.wantHelpOpen, got)
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
		probe.reply <- probedState{activeTab: probed.ActiveTab().Title, isHelpOpen: probed.helpOverlay.isOpen}
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
	program := startProgram(t, probedModel{newTestModel(t, testTabs(t, "Tasks", "Board"), nil)})

	program.Send(keyHelp)
	if state := probeState(t, program); !state.isHelpOpen {
		t.Fatal("expected the help key to open the help")
	}

	program.Send(keyQuit)
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
			model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
			refreshNow(model)
			for _, keyPress := range testCase.keys {
				if _, cmd := model.Update(keyPress); cmd != nil {
					t.Errorf("expected no command from %q", keyPress.String())
				}
			}
			activeTab := model.ActiveTab()
			if got := activeTab.FocusedTile().title; got != testCase.wantFocusedTile {
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
		{name: "a click while the help is open does nothing", msgs: []tea.Msg{keyHelp, clickMiddleThirdRow, clickSecondTabLabel}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "the wheel scrolls an unfocused tile", msgs: []tea.Msg{wheelDownTop}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"1/5", "2/5", "1/5", "1/5"}, golden: "screen_mouse_wheel_scrolled"},
		{name: "a click on a scrolled tile counts from the first row in sight", msgs: []tea.Msg{wheelDownTop, clickTopSecondLine}, wantActiveTab: "First", wantFocusedTile: "Top", wantStatuses: []string{"1/5", "3/5", "1/5", "1/5"}},
		{name: "the wheel stops at the last row", msgs: []tea.Msg{wheelDownTop, wheelDownTop, wheelDownTop, clickTopFirstLine}, wantActiveTab: "First", wantFocusedTile: "Top", wantStatuses: []string{"1/5", "2/5", "1/5", "1/5"}},
		{name: "the wheel scrolls back up", msgs: []tea.Msg{wheelDownTop, wheelUpTop, clickTopFirstLine}, wantActiveTab: "First", wantFocusedTile: "Top", wantStatuses: []string{"1/5", "1/5", "1/5", "1/5"}},
		{name: "a click at the cell of another tile goes to the zoomed tile", msgs: []tea.Msg{keyZoom, clickTopSecondLine}, wantActiveTab: "First", wantFocusedTile: "Left", wantStatuses: []string{"2/5"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := Config{Tabs: []TabConfig{tallLeftConfig("First"), tallLeftConfig("Second")}}
			model := newTestModel(t, buildTestTabs(t, config), nil)
			model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
			refreshNow(model)
			for _, msg := range testCase.msgs {
				model.Update(msg)
			}
			activeTab := model.ActiveTab()
			if got := activeTab.Title; got != testCase.wantActiveTab {
				t.Errorf("expected tab %q active, got %q", testCase.wantActiveTab, got)
			}
			if got := activeTab.FocusedTile().title; got != testCase.wantFocusedTile {
				t.Errorf("expected focus on tile %q, got %q", testCase.wantFocusedTile, got)
			}
			if got := activeTab.Statuses(); !slices.Equal(got, testCase.wantStatuses) {
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
			model.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
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
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
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
		if got := model.ActiveTab().FocusedTile().title; got != testCase.wantFocusedTile {
			t.Errorf("expected tab %q to keep focus on tile %q, got %q", model.ActiveTab().Title, testCase.wantFocusedTile, got)
		}
	}
}

func TestModel_SkipsRefreshWhileLoadRuns(t *testing.T) {
	source := &fakeTaskSource{tasks: tuitest.ListedTasks(), startedCalls: make(chan int, 100), release: make(chan struct{})}
	refreshInterval := time.Duration(minDataRefreshSeconds * float64(time.Second))
	program := startProgram(t, newTestModelWithSource(t, source, testTabs(t, "Tasks"), refreshInterval, nil))

	waitForCall(t, source.startedCalls, 1)
	program.Send(keyRefresh)
	select {
	case number := <-source.startedCalls:
		t.Fatalf("call %d started while call 1 still ran", number)
	case <-time.After(4 * refreshInterval):
	}

	close(source.release)
	waitForCall(t, source.startedCalls, 2)
}

func TestModel_DrudgerSchedule(t *testing.T) {
	const shortInterval = 10 * time.Millisecond
	cases := []struct {
		name      string
		intervals Intervals
		keys      []tea.KeyPressMsg
		// wantReads and wantChecks are the calls each source starts. A count
		// above 1 comes from the ticks of its interval or from the keys.
		wantReads  int
		wantChecks int
	}{
		{name: "reads follow the data refresh", intervals: Intervals{Refresh: shortInterval, DrudgerCheck: testRefreshInterval}, wantReads: 3, wantChecks: 1},
		{name: "checks follow the Drudger check", intervals: Intervals{Refresh: testRefreshInterval, DrudgerCheck: shortInterval}, wantReads: 1, wantChecks: 3},
		{name: "R checks right away", intervals: Intervals{Refresh: testRefreshInterval, DrudgerCheck: testRefreshInterval}, keys: []tea.KeyPressMsg{keyCheck}, wantReads: 1, wantChecks: 2},
		{name: "r reads and does not check", intervals: Intervals{Refresh: testRefreshInterval, DrudgerCheck: testRefreshInterval}, keys: []tea.KeyPressMsg{keyRefresh}, wantReads: 2, wantChecks: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			drudgers := &fakeDrudgerSource{startedReads: make(chan int, 100), startedChecks: make(chan int, 100)}
			sources := Sources{Tasks: &fakeTaskSource{tasks: tuitest.ListedTasks()}, Drudgers: drudgers}
			program := startProgram(t, newTestModelWithSources(t, sources, testTabs(t, "Tasks"), testCase.intervals, nil))

			waitForCall(t, drudgers.startedReads, 1)
			waitForCall(t, drudgers.startedChecks, 1)
			for _, keyPress := range testCase.keys {
				program.Send(keyPress)
			}
			assertCalls(t, "reads", drudgers.startedReads, &drudgers.readCount, testCase.wantReads)
			assertCalls(t, "checks", drudgers.startedChecks, &drudgers.checkCount, testCase.wantChecks)
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

func TestModel_SkipsCheckWhileCheckRuns(t *testing.T) {
	drudgers := &fakeDrudgerSource{startedChecks: make(chan int, 100), releaseChecks: make(chan struct{})}
	sources := Sources{Tasks: &fakeTaskSource{tasks: tuitest.ListedTasks()}, Drudgers: drudgers}
	checkInterval := 10 * time.Millisecond
	program := startProgram(t, newTestModelWithSources(t, sources, testTabs(t, "Tasks"), Intervals{Refresh: testRefreshInterval, DrudgerCheck: checkInterval}, nil))

	waitForCall(t, drudgers.startedChecks, 1)
	program.Send(keyCheck)
	select {
	case number := <-drudgers.startedChecks:
		t.Fatalf("check %d started while check 1 still ran", number)
	case <-time.After(4 * checkInterval):
	}

	close(drudgers.releaseChecks)
	waitForCall(t, drudgers.startedChecks, 2)
}

func TestModel_FailedCheckShowsError(t *testing.T) {
	drudgers := &fakeDrudgerSource{checkErr: errDiskOnFire}
	sources := Sources{Tasks: &fakeTaskSource{tasks: tuitest.ListedTasks()}, Drudgers: drudgers}
	model := newTestModelWithSources(t, sources, testTabs(t, "Tasks"), Intervals{Refresh: testRefreshInterval, DrudgerCheck: testRefreshInterval}, nil)
	model.Update(tea.WindowSizeMsg{Width: 140, Height: 12})

	_, cmd := model.Update(keyCheck)
	runCommand(model, cmd)

	if wantError := "✗ could not check Drudgers: disk on fire"; !strings.Contains(model.View().Content, wantError) {
		t.Errorf("expected the screen to show %q, got:\n%s", wantError, model.View().Content)
	}
}

func TestModel_FailedLoadKeepsLastData(t *testing.T) {
	source := &fakeTaskSource{tasks: tuitest.ListedTasks(), failFromCall: 2}
	model := newTestModelWithSource(t, source, testTabs(t, "Tasks"), testRefreshInterval, nil)
	model.Update(tea.WindowSizeMsg{Width: 140, Height: 12})

	refreshNow(model)
	refreshNow(model)

	content := model.View().Content
	if wantTask := "Add retry to uploader"; !strings.Contains(content, wantTask) {
		t.Errorf("expected the tasks of the first load to stay on screen, got:\n%s", content)
	}
	if wantError := "✗ could not load tasks: disk on fire"; !strings.Contains(content, wantError) {
		t.Errorf("expected the screen to show %q, got:\n%s", wantError, content)
	}
}

func TestModel_SuccessfulLoadClearsError(t *testing.T) {
	source := &fakeTaskSource{tasks: tuitest.ListedTasks(), failFromCall: 1}
	model := newTestModelWithSource(t, source, testTabs(t, "Tasks"), testRefreshInterval, nil)
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 12})

	refreshNow(model)
	source.failFromCall = 0
	refreshNow(model)

	if content := model.View().Content; strings.Contains(content, "could not load") {
		t.Errorf("expected no load error after a load that worked, got:\n%s", content)
	}
}

func TestModel_BottomBar(t *testing.T) {
	threeTiles := func(middleShowStatus *bool) Config {
		return oneTabConfig(NodeConfig{Arrangement: Stacked, Children: []NodeConfig{
			{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), {View: tasklist.Name, ShowStatus: middleShowStatus}}},
			taskListNode(),
		}})
	}
	cases := []struct {
		name         string
		width        int
		config       Config
		failFromCall int
		warnings     []string
		keys         []tea.KeyPressMsg
	}{
		{name: "bottom_bar_segments", width: 140, config: threeTiles(nil), keys: []tea.KeyPressMsg{tuitest.KeyDown, tuitest.KeyDown}},
		{name: "bottom_bar_status_hidden", width: 140, config: threeTiles(new(false)), keys: []tea.KeyPressMsg{tuitest.KeyDown, tuitest.KeyDown}},
		{
			name:         "bottom_bar_errors_and_warnings",
			width:        240,
			config:       threeTiles(nil),
			failFromCall: 2,
			warnings:     []string{`theme.json: "red" is not a color for role error, using the theme's own`},
		},
		{name: "bottom_bar_overflow_cuts_segments", width: 124, config: threeTiles(nil), failFromCall: 2},
		{name: "bottom_bar_overflow_cuts_errors", width: 100, config: threeTiles(nil), failFromCall: 2},
		{
			name:         "bottom_bar_zoomed",
			width:        280,
			config:       threeTiles(nil),
			failFromCall: 2,
			warnings:     []string{`theme.json: "red" is not a color for role error, using the theme's own`},
			keys:         []tea.KeyPressMsg{keyNextTile, tuitest.KeyDown, keyZoom},
		},
		{name: "bottom_bar_zoomed_status_hidden", width: 140, config: threeTiles(new(false)), keys: []tea.KeyPressMsg{keyNextTile, keyZoom}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			source := &fakeTaskSource{tasks: tuitest.ListedTasks(), failFromCall: testCase.failFromCall}
			model := newTestModelWithSource(t, source, buildTestTabs(t, testCase.config), testRefreshInterval, testCase.warnings)
			model.Update(tea.WindowSizeMsg{Width: testCase.width, Height: 24})
			refreshNow(model)
			refreshNow(model)
			for _, keyPress := range testCase.keys {
				model.Update(keyPress)
			}
			lines := strings.Split(model.View().Content, "\n")
			tuitest.AssertGolden(t, testCase.name, lines[len(lines)-1])
		})
	}
}

func TestNewModel_NoTabs(t *testing.T) {
	sources := Sources{Tasks: &fakeTaskSource{}, Drudgers: &fakeDrudgerSource{}}
	_, err := NewModel(testProject, sources, tuitest.Styles(), nil, Intervals{Refresh: testRefreshInterval, DrudgerCheck: testRefreshInterval}, true, nil)
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
