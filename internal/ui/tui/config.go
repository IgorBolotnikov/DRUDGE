package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

// Titles of the bundled Tabs.
const (
	overviewTabTitle = "overview"
	tasksTabTitle    = "tasks"
)

// tabsPath is the path of the Tabs in the errors about them.
const tabsPath = "tabs"

// viewNameSeparator joins the names of the known Views in an error.
const viewNameSeparator = ", "

// Config describes the Tabs of the TUI and the layout of each Tab. It holds
// only plain values, so building it again gives fresh Tabs.
type Config struct {
	Tabs []TabConfig
}

// TabConfig describes one Tab.
type TabConfig struct {
	Title  string
	Layout NodeConfig
}

// NodeConfig describes one node of a layout. A split sets Arrangement and
// Children. A Tile sets View and may set Title. A weight of 0 is the default
// weight.
type NodeConfig struct {
	Arrangement Arrangement
	Children    []NodeConfig
	View        string
	Title       string
	Weight      float64
}

// defaultConfig returns the Tabs the TUI shows when there is no TUI config
// file.
func defaultConfig() Config {
	return Config{Tabs: []TabConfig{
		{Title: overviewTabTitle, Layout: NodeConfig{Arrangement: Stacked, Children: []NodeConfig{
			{Arrangement: SideBySide, Children: []NodeConfig{
				{View: tasklist.Name, Weight: 2},
				{View: tasklist.Name},
			}},
			{Arrangement: SideBySide, Children: []NodeConfig{
				{View: tasklist.Name},
				{View: tasklist.Name},
			}},
		}}},
		{Title: tasksTabTitle, Layout: NodeConfig{View: tasklist.Name}},
	}}
}

// BuildTabs builds new Tabs from a config, each Tile with a View of its own.
// It refuses a config with no Tabs, a Tab with no title or with the title of
// another Tab, and a layout it cannot build. An error starts with the path of
// the value to change, like tabs[0].layout.children[1].view.
func BuildTabs(config Config, screenStyles *styles.Styles) ([]*Tab, error) {
	if len(config.Tabs) == 0 {
		return nil, fmt.Errorf("%s: there are no tabs, add at least one tab", tabsPath)
	}
	tabs := make([]*Tab, 0, len(config.Tabs))
	tabIndexByTitle := make(map[string]int, len(config.Tabs))
	for index, tabConfig := range config.Tabs {
		if firstIndex, isTaken := tabIndexByTitle[tabConfig.Title]; isTaken {
			return nil, fmt.Errorf("%s.title: %q is already the title of %s, give each tab its own title", tabPath(index), tabConfig.Title, tabPath(firstIndex))
		}
		tabIndexByTitle[tabConfig.Title] = index
		tab, err := buildTab(tabConfig, screenStyles)
		if err != nil {
			return nil, fmt.Errorf("%s.%w", tabPath(index), err)
		}
		tabs = append(tabs, tab)
	}
	return tabs, nil
}

// tabPath returns the path of the Tab at index.
func tabPath(index int) string {
	return fmt.Sprintf("%s[%d]", tabsPath, index)
}

func buildTab(tabConfig TabConfig, screenStyles *styles.Styles) (*Tab, error) {
	if tabConfig.Title == "" {
		return nil, errors.New("title: the tab has no title, give it one")
	}
	layout, err := buildNode(tabConfig.Layout, layoutPath, screenStyles)
	if err != nil {
		return nil, err
	}
	return NewTab(tabConfig.Title, layout)
}

// buildNode builds the Node a config describes. It refuses a node that is both
// a split and a Tile or neither, a split with a title and a Tile with an
// unknown View. NewTab checks the rest.
func buildNode(nodeConfig NodeConfig, path string, screenStyles *styles.Styles) (Node, error) {
	weight := nodeConfig.Weight
	if weight == 0 {
		weight = defaultWeight
	}
	isSplit := nodeConfig.Arrangement != "" || len(nodeConfig.Children) > 0
	isTile := nodeConfig.View != ""
	switch {
	case isSplit && isTile:
		return nil, fmt.Errorf("%s: the node sets both a view and a split, keep view for a tile or arrangement and children for a split", path)
	case isTile:
		return buildTile(nodeConfig, weight, path, screenStyles)
	case isSplit:
		return buildSplit(nodeConfig, weight, path, screenStyles)
	default:
		return nil, fmt.Errorf("%s: the node is empty, set view for a tile or arrangement and children for a split", path)
	}
}

func buildTile(nodeConfig NodeConfig, weight float64, path string, screenStyles *styles.Styles) (*Tile, error) {
	kind, ok := findViewKind(nodeConfig.View)
	if !ok {
		return nil, fmt.Errorf("%s.view: unknown view %q, use one of: %s", path, nodeConfig.View, strings.Join(viewKindNames(), viewNameSeparator))
	}
	return NewTile(kind.newView(screenStyles), nodeConfig.Title, screenStyles, weight), nil
}

func buildSplit(nodeConfig NodeConfig, weight float64, path string, screenStyles *styles.Styles) (*Split, error) {
	if nodeConfig.Title != "" {
		return nil, fmt.Errorf("%s.title: only a tile has a title, move it to a tile of the split", path)
	}
	children := make([]Node, 0, len(nodeConfig.Children))
	for index, childConfig := range nodeConfig.Children {
		child, err := buildNode(childConfig, childPath(path, index), screenStyles)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return NewSplit(nodeConfig.Arrangement, weight, children...), nil
}
