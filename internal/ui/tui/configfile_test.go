package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

const (
	referencePage     = "../../../site/content/docs/configuration/tui.md"
	tuiPage           = "../../../site/content/docs/tui/_index.md"
	regenerateCommand = "go test ./internal/ui/tui -update"
	jsonFenceOpen     = "```json\n"
	fenceClose        = "```"
)

func TestParseConfig(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		want    Config
		wantErr string
	}{
		{
			name: "a tab with a tile",
			data: `{"$schema": "./schema/tui.json", "tabs": [{"title": "Tasks", "layout": {"view": "task-list", "title": "Mine", "weight": 1.5}}]}`,
			want: Config{Tabs: []TabConfig{{Title: "Tasks", Layout: NodeConfig{View: tasklist.Name, Title: "Mine", Weight: 1.5}}}},
		},
		{
			name: "a split",
			data: `{"tabs": [{"title": "Both", "layout": {"arrangement": "side-by-side", "weight": 2, "children": [{"view": "task-list"}, {"view": "task-list", "weight": 0.5}]}}]}`,
			want: Config{Tabs: []TabConfig{{Title: "Both", Layout: NodeConfig{Arrangement: SideBySide, Weight: 2, Children: []NodeConfig{
				{View: tasklist.Name},
				{View: tasklist.Name, Weight: 0.5},
			}}}}},
		},
		{
			name: "a missing weight keeps the default",
			data: `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list"}}]}`,
			want: Config{Tabs: []TabConfig{{Title: "Tasks", Layout: NodeConfig{View: tasklist.Name}}}},
		},
		{
			name: "a data refresh in whole seconds",
			data: `{"dataRefreshSeconds": 5}`,
			want: Config{DataRefreshSeconds: 5},
		},
		{
			name: "a data refresh that is a fraction",
			data: `{"dataRefreshSeconds": 0.5}`,
			want: Config{DataRefreshSeconds: 0.5},
		},
		{
			name: "a data refresh at the minimum",
			data: `{"dataRefreshSeconds": 0.1}`,
			want: Config{DataRefreshSeconds: 0.1},
		},
		{
			name: "a Drudger check",
			data: `{"drudgerCheckSeconds": 60}`,
			want: Config{DrudgerCheckSeconds: 60},
		},
		{
			name: "a Drudger check at the minimum",
			data: `{"drudgerCheckSeconds": 1}`,
			want: Config{DrudgerCheckSeconds: 1},
		},
		{
			name: "mouse capture off",
			data: `{"captureMouse": false}`,
			want: Config{CaptureMouse: new(false)},
		},
		{
			name: "mouse capture on",
			data: `{"captureMouse": true}`,
			want: Config{CaptureMouse: new(true)},
		},
		{
			name: "a tile that hides its status",
			data: `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "showStatus": false}}]}`,
			want: Config{Tabs: []TabConfig{{Title: "Tasks", Layout: NodeConfig{View: tasklist.Name, ShowStatus: new(false)}}}},
		},
		{
			name: "a tile that shows its status",
			data: `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "showStatus": true}}]}`,
			want: Config{Tabs: []TabConfig{{Title: "Tasks", Layout: NodeConfig{View: tasklist.Name, ShowStatus: new(true)}}}},
		},
		{
			name: "a tile with view options",
			data: `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "viewOptions": {"status": "todo", "colour": 1}}}]}`,
			want: Config{Tabs: []TabConfig{{Title: "Tasks", Layout: NodeConfig{View: tasklist.Name, ViewOptions: map[string]any{"status": "todo", "colour": float64(1)}}}}},
		},
		{
			name:    "view options that are an array",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "viewOptions": ["todo"]}}]}`,
			wantErr: "tabs[0].layout.viewOptions: expected an object, got an array",
		},
		{
			name:    "a data refresh below the minimum",
			data:    `{"dataRefreshSeconds": 0.05}`,
			wantErr: "dataRefreshSeconds: 0.05 is below the minimum of 0.1, use 0.1 or more seconds",
		},
		{
			name:    "a data refresh of 0",
			data:    `{"dataRefreshSeconds": 0}`,
			wantErr: "dataRefreshSeconds: 0 is below the minimum of 0.1, use 0.1 or more seconds",
		},
		{
			name:    "a data refresh that is a string",
			data:    `{"dataRefreshSeconds": "2"}`,
			wantErr: "dataRefreshSeconds: expected a number, got a string",
		},
		{
			name:    "a Drudger check below the minimum",
			data:    `{"drudgerCheckSeconds": 0}`,
			wantErr: "drudgerCheckSeconds: 0 is below the minimum of 1, use 1 or more seconds",
		},
		{
			name:    "a Drudger check that is a fraction",
			data:    `{"drudgerCheckSeconds": 2.5}`,
			wantErr: "drudgerCheckSeconds: 2.5 is not a whole number, use whole seconds like 30",
		},
		{
			name:    "a Drudger check that is a string",
			data:    `{"drudgerCheckSeconds": "30"}`,
			wantErr: "drudgerCheckSeconds: expected a number, got a string",
		},
		{
			name:    "a mouse capture that is a string",
			data:    `{"captureMouse": "no"}`,
			wantErr: "captureMouse: expected a boolean, got a string",
		},
		{
			name:    "a show status that is a string",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "showStatus": "no"}}]}`,
			wantErr: "tabs[0].layout.showStatus: expected a boolean, got a string",
		},
		{
			name: "an empty object",
			data: `{}`,
			want: Config{},
		},
		{
			name:    "a file that does not parse",
			data:    `{"tabs": [`,
			wantErr: "the file is not valid JSON: unexpected end of JSON input",
		},
		{
			name:    "a file that is not an object",
			data:    `[]`,
			wantErr: "expected an object, got an array",
		},
		{
			name:    "an unknown key at the top",
			data:    `{"tab": []}`,
			wantErr: "tab: unknown key, use one of: $schema, tabs, dataRefreshSeconds, drudgerCheckSeconds, captureMouse",
		},
		{
			name:    "an unknown key in a tab",
			data:    `{"tabs": [{"title": "Tasks", "name": "Tasks"}]}`,
			wantErr: "tabs[0].name: unknown key, use one of: title, layout",
		},
		{
			name:    "an unknown key deep in a layout",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"arrangement": "stacked", "children": [{"view": "task-list"}, {"view": "task-list", "colour": "red"}]}}]}`,
			wantErr: "tabs[0].layout.children[1].colour: unknown key, use one of: arrangement, children, view, title, weight, showStatus, viewOptions",
		},
		{
			name:    "an explicit weight of 0",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"arrangement": "stacked", "children": [{"view": "task-list", "weight": 0}, {"view": "task-list"}]}}]}`,
			wantErr: "tabs[0].layout.children[0].weight: 0 is not above 0, use a positive weight",
		},
		{
			name:    "a negative weight",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "weight": -1}}]}`,
			wantErr: "tabs[0].layout.weight: -1 is not above 0, use a positive weight",
		},
		{
			name:    "a weight of null",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "weight": null}}]}`,
			wantErr: "tabs[0].layout.weight: expected a number, got null",
		},
		{
			name:    "a weight that is a string",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "weight": "2"}}]}`,
			wantErr: "tabs[0].layout.weight: expected a number, got a string",
		},
		{
			name:    "a schema that is not a string",
			data:    `{"$schema": true}`,
			wantErr: "$schema: expected a string, got a boolean",
		},
		{
			name:    "tabs that are an object",
			data:    `{"tabs": {"title": "Tasks"}}`,
			wantErr: "tabs: expected an array, got an object",
		},
		{
			name:    "a tab that is a string",
			data:    `{"tabs": ["Tasks"]}`,
			wantErr: "tabs[0]: expected an object, got a string",
		},
		{
			name:    "a title that is a number",
			data:    `{"tabs": [{"title": 1}]}`,
			wantErr: "tabs[0].title: expected a string, got a number",
		},
		{
			name:    "an arrangement that is a number",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"arrangement": 1}}]}`,
			wantErr: "tabs[0].layout.arrangement: expected a string, got a number",
		},
		{
			name:    "children that are an object",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"arrangement": "stacked", "children": {}}}]}`,
			wantErr: "tabs[0].layout.children: expected an array, got an object",
		},
		{
			name:    "a view that is an array",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"view": ["task-list"]}}]}`,
			wantErr: "tabs[0].layout.view: expected a string, got an array",
		},
		{
			name:    "a tile title that is a boolean",
			data:    `{"tabs": [{"title": "Tasks", "layout": {"view": "task-list", "title": false}}]}`,
			wantErr: "tabs[0].layout.title: expected a string, got a boolean",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ParseConfig([]byte(testCase.data))
			if testCase.wantErr != "" {
				if err == nil || err.Error() != testCase.wantErr {
					t.Fatalf("expected the error %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, got)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cases := []struct {
		name    string
		data    *string
		want    Config
		wantErr string
	}{
		{name: "a missing file gives the default", want: defaultConfig()},
		{
			name: "a file with tabs",
			data: new(`{"tabs": [{"title": "Tasks", "layout": {"view": "task-list"}}]}`),
			want: Config{Tabs: []TabConfig{{Title: "Tasks", Layout: NodeConfig{View: tasklist.Name}}}},
		},
		{name: "a file that does not parse", data: new(`{`), wantErr: ": the file is not valid JSON"},
		{name: "a file with an unknown key", data: new(`{"tab": []}`), wantErr: ": tab: unknown key"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), common.TuiConfigName)
			if testCase.data != nil {
				if err := os.WriteFile(path, []byte(*testCase.data), common.DefaultFilePerm); err != nil {
					t.Fatal(err)
				}
			}
			got, err := LoadConfig(path)
			if testCase.wantErr != "" {
				if err == nil || !strings.HasPrefix(err.Error(), path+testCase.wantErr) {
					t.Fatalf("expected an error starting with %q, got %v", path+testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, got)
			}
		})
	}
}

func TestDocsExampleIsDefaultConfig(t *testing.T) {
	page, err := os.ReadFile(tuiPage)
	if err != nil {
		t.Fatalf("could not read the TUI page: %v", err)
	}
	_, afterFence, ok := strings.Cut(string(page), jsonFenceOpen)
	if !ok {
		t.Fatalf("%s has no JSON example", tuiPage)
	}
	example, _, ok := strings.Cut(afterFence, fenceClose)
	if !ok {
		t.Fatalf("the JSON example of %s is not closed", tuiPage)
	}
	got, err := ParseConfig([]byte(example))
	if err != nil {
		t.Fatalf("the JSON example of %s does not parse: %v", tuiPage, err)
	}
	if !reflect.DeepEqual(got, defaultConfig()) {
		t.Errorf("the JSON example of %s is not the bundled default\ngot:  %+v\nwant: %+v", tuiPage, got, defaultConfig())
	}
}

func TestReferencePageMatchesSchema(t *testing.T) {
	page, err := os.ReadFile(referencePage)
	if err != nil {
		t.Fatalf("could not read the reference page: %v", err)
	}
	table, err := common.RenderSchemaTable(Schema())
	if err != nil {
		t.Fatalf("could not render the schema: %v", err)
	}
	rendered, err := common.SpliceGenerated(string(page), table)
	if err != nil {
		t.Fatalf("%s: %v", referencePage, err)
	}

	if tuitest.ShouldUpdate() {
		if err := os.WriteFile(referencePage, []byte(rendered), common.DefaultFilePerm); err != nil {
			t.Fatalf("could not write the reference page: %v", err)
		}
		return
	}
	if string(page) != rendered {
		t.Errorf("the generated table of %s does not match its schema, run `%s` to regenerate it", referencePage, regenerateCommand)
	}
}

func TestRun_RefusesInvalidConfigBeforeAltScreen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	path := common.TuiConfigPath(home)
	if err := common.EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"tabs": [{"title": "Tasks", "layout": {"view": "kanban"}}]}`), common.DefaultFilePerm); err != nil {
		t.Fatal(err)
	}

	err := Run()

	wantErr := path + `: tabs[0].layout.view: unknown view "kanban"`
	if err == nil || !strings.HasPrefix(err.Error(), wantErr) {
		t.Fatalf("expected an error starting with %q, got %v", wantErr, err)
	}
}
