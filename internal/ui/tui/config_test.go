package tui

import (
	"maps"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
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
		{name: "unknown view", config: oneTabConfig(NodeConfig{View: "kanban"}), wantPath: "tabs[0].layout.view: ", wantReason: `unknown view "kanban", use one of: drudger-list, task-board, task-detail, task-list`},
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
			name:       "split with a status setting",
			config:     oneTabConfig(NodeConfig{Arrangement: Stacked, ShowStatus: new(false), Children: []NodeConfig{taskListNode(), taskListNode()}}),
			wantPath:   "tabs[0].layout.showStatus: ",
			wantReason: "only a tile has a status",
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

func TestBuildTabs_ViewOptions(t *testing.T) {
	inSplit := func(viewOptions map[string]any) Config {
		return oneTabConfig(NodeConfig{Arrangement: Stacked, Children: []NodeConfig{
			taskListNode(),
			{View: tasklist.Name, ViewOptions: viewOptions},
		}})
	}
	cases := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{name: "no view options", config: inSplit(nil)},
		{name: "empty view options", config: inSplit(map[string]any{})},
		{name: "a known status", config: inSplit(map[string]any{"status": "done"})},
		{
			name:    "an unknown option",
			config:  inSplit(map[string]any{"colour": "red"}),
			wantErr: `tabs[0].layout.children[1].viewOptions.colour: view "task-list" has no option "colour", use one of: status`,
		},
		{
			name:    "an unknown status",
			config:  inSplit(map[string]any{"status": "blocked"}),
			wantErr: `tabs[0].layout.children[1].viewOptions.status: "blocked" is not allowed, use one of: draft, todo, in-progress, fucked-up, unmerged, done`,
		},
		{
			name:    "a status that is a number",
			config:  inSplit(map[string]any{"status": float64(1)}),
			wantErr: "tabs[0].layout.children[1].viewOptions.status: expected a string, got a number",
		},
		{
			name:    "a status of null",
			config:  inSplit(map[string]any{"status": nil}),
			wantErr: "tabs[0].layout.children[1].viewOptions.status: expected a string, got null",
		},
		{
			name:    "view options on a split",
			config:  oneTabConfig(NodeConfig{Arrangement: Stacked, ViewOptions: map[string]any{}, Children: []NodeConfig{taskListNode(), taskListNode()}}),
			wantErr: "tabs[0].layout.viewOptions: only a tile has view options, move them to a tile of the split",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := BuildTabs(testCase.config, tuitest.Styles())
			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != testCase.wantErr {
				t.Fatalf("expected the error %q, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestConfig_Intervals(t *testing.T) {
	cases := []struct {
		name   string
		config Config
		want   map[views.FeedName]time.Duration
	}{
		{
			name: "unset gives the defaults",
			want: map[views.FeedName]time.Duration{views.TasksFeed: 2 * time.Second, views.DrudgersFeed: 2 * time.Second, views.DrudgerCheckFeed: 30 * time.Second, views.SessionsFeed: 5 * time.Second, views.RunLogFeed: time.Second},
		},
		{
			name:   "a missing feed takes its default",
			config: Config{RefreshSeconds: map[views.FeedName]float64{views.TasksFeed: 0.25, views.DrudgerCheckFeed: 10}},
			want:   map[views.FeedName]time.Duration{views.TasksFeed: 250 * time.Millisecond, views.DrudgersFeed: 2 * time.Second, views.DrudgerCheckFeed: 10 * time.Second, views.SessionsFeed: 5 * time.Second, views.RunLogFeed: time.Second},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.config.Intervals(); !maps.Equal(got, testCase.want) {
				t.Errorf("expected %v, got %v", testCase.want, got)
			}
		})
	}
}

func TestConfig_MouseMode(t *testing.T) {
	cases := []struct {
		name         string
		captureMouse *bool
		want         tea.MouseMode
	}{
		{name: "unset captures the mouse", want: tea.MouseModeCellMotion},
		{name: "on captures the mouse", captureMouse: new(true), want: tea.MouseModeCellMotion},
		{name: "off leaves mouse mode off", captureMouse: new(false), want: tea.MouseModeNone},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := oneTabConfig(taskListNode())
			config.CaptureMouse = testCase.captureMouse
			model, err := NewModel(testProject, feeds.Sources{Tasks: &fakeTaskSource{}, Drudgers: &fakeDrudgerSource{}}, newTestCommands(&fakeCommandService{}), tuitest.Styles(), buildTestTabs(t, config), config.Intervals(), config.IsMouseCaptured(), nil)
			if err != nil {
				t.Fatal(err)
			}
			model.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
			if got := model.View().MouseMode; got != testCase.want {
				t.Errorf("expected mouse mode %v, got %v", testCase.want, got)
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
			name:   "screen_status_option",
			width:  100,
			height: 12,
			config: oneTabConfig(NodeConfig{View: tasklist.Name, ViewOptions: map[string]any{tasklist.StatusOption: "done"}}),
		},
		{
			name:   "screen_too_small_for_tiles",
			width:  50,
			height: 12,
			config: oneTabConfig(nestedLayout),
		},
		{
			name:   "screen_too_small_zoomed",
			width:  50,
			height: 12,
			config: oneTabConfig(nestedLayout),
			keys:   []tea.KeyPressMsg{keyZoom},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newTestModel(t, buildTestTabs(t, testCase.config), nil)
			update(model, tea.WindowSizeMsg{Width: testCase.width, Height: testCase.height})
			startFeeds(model)
			for _, keyPress := range testCase.keys {
				model.Update(keyPress)
			}
			tuitest.AssertGolden(t, testCase.name, model.View().Content)
		})
	}
}
