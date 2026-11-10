package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const testProjectSlug = "uploader"

type fakeTaskSource struct {
	tasks []task.ListedTask
}

func (source fakeTaskSource) ListTasks(projectSlug string, filter task.ListTasksFilter, page int, size int) (common.Page[task.ListedTask], error) {
	return common.Paginate(source.tasks, page, size)
}

func TestModel(t *testing.T) {
	cases := []struct {
		name     string
		width    int
		height   int
		warnings []string
		keys     []tea.KeyPressMsg
	}{
		{name: "screen", width: 100, height: 12},
		{name: "screen_cursor_moved", width: 100, height: 12, keys: []tea.KeyPressMsg{keyDown, keyDown}},
		{name: "screen_with_warning", width: 100, height: 12, warnings: []string{`theme.json: "red" is not a color for role error, using the theme's own`}},
		{name: "screen_short", width: 100, height: 5},
		{name: "screen_narrow", width: 30, height: 8},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := NewModel(testProjectSlug, fakeTaskSource{tasks: listedTasks()}, testStyles(), testCase.warnings)
			model.Update(tea.WindowSizeMsg{Width: testCase.width, Height: testCase.height})
			model.Update(model.Init()())
			for _, keyPress := range testCase.keys {
				model.Update(keyPress)
			}

			view := model.View()
			if !view.AltScreen {
				t.Error("expected the screen in the alt screen")
			}
			lines := strings.Split(view.Content, "\n")
			if len(lines) != testCase.height {
				t.Errorf("expected %d lines, got %d", testCase.height, len(lines))
			}
			assertGolden(t, testCase.name, view.Content)
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
			model := NewModel(testProjectSlug, fakeTaskSource{}, testStyles(), nil)
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
