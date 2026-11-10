package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

// taskListNode describes a Tile with a task list.
func taskListNode() NodeConfig {
	return NodeConfig{View: tasklist.Name}
}

// oneTabConfig describes a single Tab with a layout.
func oneTabConfig(layout NodeConfig) Config {
	return Config{Tabs: []TabConfig{{Title: "Tab", Layout: layout}}}
}

func TestBuildTabs_RefusesConfig(t *testing.T) {
	cases := []struct {
		name       string
		config     Config
		wantPath   string
		wantReason string
	}{
		{name: "no tabs", config: Config{}, wantPath: "tabs: ", wantReason: "add at least one tab"},
		{
			name:       "tab with no title",
			config:     Config{Tabs: []TabConfig{{Layout: taskListNode()}}},
			wantPath:   "tabs[0].title: ",
			wantReason: "has no title",
		},
		{
			name:       "two tabs with one title",
			config:     Config{Tabs: []TabConfig{{Title: "Tasks", Layout: taskListNode()}, {Title: "Board", Layout: taskListNode()}, {Title: "Tasks", Layout: taskListNode()}}},
			wantPath:   "tabs[2].title: ",
			wantReason: `"Tasks" is already the title of tabs[0]`,
		},
		{name: "empty node", config: oneTabConfig(NodeConfig{}), wantPath: "tabs[0].layout: ", wantReason: "the node is empty"},
		{
			name:       "node that is a split and a tile",
			config:     oneTabConfig(NodeConfig{View: tasklist.Name, Arrangement: Stacked, Children: []NodeConfig{taskListNode(), taskListNode()}}),
			wantPath:   "tabs[0].layout: ",
			wantReason: "sets both a view and a split",
		},
		{name: "unknown view", config: oneTabConfig(NodeConfig{View: "kanban"}), wantPath: "tabs[0].layout.view: ", wantReason: `unknown view "kanban", use one of: task-list`},
		{
			name: "unknown view deep in the tree",
			config: oneTabConfig(NodeConfig{Arrangement: Stacked, Children: []NodeConfig{
				{Arrangement: SideBySide, Children: []NodeConfig{taskListNode(), {View: "kanban"}}},
				taskListNode(),
			}}),
			wantPath:   "tabs[0].layout.children[0].children[1].view: ",
			wantReason: `unknown view "kanban"`,
		},
		{
			name:       "split with a title",
			config:     oneTabConfig(NodeConfig{Arrangement: Stacked, Title: "Both", Children: []NodeConfig{taskListNode(), taskListNode()}}),
			wantPath:   "tabs[0].layout.title: ",
			wantReason: "only a tile has a title",
		},
		{
			name:       "split with one child",
			config:     oneTabConfig(NodeConfig{Arrangement: Stacked, Children: []NodeConfig{taskListNode()}}),
			wantPath:   "tabs[0].layout.children: ",
			wantReason: "needs at least 2",
		},
		{
			name:       "children with no arrangement",
			config:     oneTabConfig(NodeConfig{Children: []NodeConfig{taskListNode(), taskListNode()}}),
			wantPath:   "tabs[0].layout.arrangement: ",
			wantReason: `unknown arrangement ""`,
		},
		{
			name:       "negative weight",
			config:     oneTabConfig(NodeConfig{Arrangement: Stacked, Children: []NodeConfig{taskListNode(), {View: tasklist.Name, Weight: -1}}}),
			wantPath:   "tabs[0].layout.children[1].weight: ",
			wantReason: "-1 is not above 0",
		},
		{
			name:       "error in a later tab",
			config:     Config{Tabs: []TabConfig{{Title: "Tasks", Layout: taskListNode()}, {Title: "Board", Layout: NodeConfig{View: "kanban"}}}},
			wantPath:   "tabs[1].layout.view: ",
			wantReason: `unknown view "kanban"`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := BuildTabs(testCase.config, tuitest.Styles())
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.HasPrefix(err.Error(), testCase.wantPath) {
				t.Errorf("expected the error to start with %q, got %q", testCase.wantPath, err)
			}
			if !strings.Contains(err.Error(), testCase.wantReason) {
				t.Errorf("expected the error to contain %q, got %q", testCase.wantReason, err)
			}
		})
	}
}

func TestBuildTabs_DefaultConfig(t *testing.T) {
	tabs := buildTestTabs(t, defaultConfig())
	if len(tabs) == 0 {
		t.Fatal("expected the default config to have tabs")
	}
}

func TestBuildTabs_Screen(t *testing.T) {
	nestedLayout := NodeConfig{Arrangement: Stacked, Children: []NodeConfig{
		{Arrangement: SideBySide, Weight: 2, Children: []NodeConfig{
			{View: tasklist.Name, Title: "Mine", Weight: 2},
			taskListNode(),
		}},
		taskListNode(),
	}}
	cases := []struct {
		name   string
		width  int
		height int
		config Config
		keys   []tea.KeyPressMsg
	}{
		{
			name:   "screen_split_tiles_own_cursor",
			width:  120,
			height: 24,
			config: oneTabConfig(nestedLayout),
			keys:   []tea.KeyPressMsg{tuitest.KeyDown, tuitest.KeyDown},
		},
		{
			name:   "screen_too_small_for_tiles",
			width:  50,
			height: 12,
			config: oneTabConfig(nestedLayout),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, buildTestTabs(t, testCase.config), nil)
			model.Update(tea.WindowSizeMsg{Width: testCase.width, Height: testCase.height})
			model.Update(model.Init()())
			for _, keyPress := range testCase.keys {
				model.Update(keyPress)
			}
			tuitest.AssertGolden(t, testCase.name, model.View().Content)
		})
	}
}
