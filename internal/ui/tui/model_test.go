package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

var testProject = &project.Project{Slug: "uploader", Name: "Uploader"}

type fakeTaskSource struct {
	tasks []task.ListedTask
}

func (source fakeTaskSource) ListTasks(projectSlug string, filter task.ListTasksFilter, page int, size int) (common.Page[task.ListedTask], error) {
	return common.Paginate(source.tasks, page, size)
}

var (
	keyNextTab     = tea.KeyPressMsg{Code: ']', Text: "]"}
	keyPreviousTab = tea.KeyPressMsg{Code: '[', Text: "["}
	keyTabTwo      = tea.KeyPressMsg{Code: '2', Text: "2"}
	keyTabThree    = tea.KeyPressMsg{Code: '3', Text: "3"}
	keyTabNine     = tea.KeyPressMsg{Code: '9', Text: "9"}
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
	model, err := NewModel(testProject, fakeTaskSource{tasks: tuitest.ListedTasks()}, tuitest.Styles(), tabs, warnings)
	if err != nil {
		t.Fatal(err)
	}
	return model
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
			model.Update(model.Init()())
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

func TestNewModel_NoTabs(t *testing.T) {
	_, err := NewModel(testProject, fakeTaskSource{}, tuitest.Styles(), nil, nil)
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
