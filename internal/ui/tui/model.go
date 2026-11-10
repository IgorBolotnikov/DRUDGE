package tui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// barHeight is how many lines the top bar and the bottom bar take together.
const barHeight = 2

// warningPrefix starts a warning in the bottom bar.
const warningPrefix = "! "

// tooSmallFormat is the warning in place of a Tab that has no room for its
// Tiles.
const tooSmallFormat = "terminal too small for tab %q, resize it"

// loadErrorPrefix starts a load error in the bottom bar.
const loadErrorPrefix = "✗ "

// loadErrorFormat is the load error of one source in the bottom bar.
const loadErrorFormat = "could not load %s: %v"

// warningSeparator joins the load errors and the warnings of the bottom bar,
// and puts them after the status segments.
const warningSeparator = "  "

// statusSeparator joins the status segments of the bottom bar.
const statusSeparator = " · "

// spreadGap is how many spaces a bar keeps between its left and its right
// part.
const spreadGap = 1

// The task list loads every task of the project on one page.
const (
	firstPage    = 1
	allOnOnePage = 0
)

const windowTitleFormat = "DRUDGE | %s"

// Labels of a Tab in the top bar. The first nine Tabs show their number key.
const (
	numberedTabLabelFormat = " %d %s "
	tabLabelFormat         = " %s "
	numberedTabCount       = 9
)

// Keys of the root. Views never bind them.
var (
	quitKey        = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit"))
	refreshKey     = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	tabNumberKey   = key.NewBinding(key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"), key.WithHelp("1-9", "tab"))
	nextTabKey     = key.NewBinding(key.WithKeys("]"), key.WithHelp("[/]", "prev/next tab"))
	previousTabKey = key.NewBinding(key.WithKeys("["))
)

// rootKeyHints are the root keys the bottom bar shows after the keys of the
// focused View.
var rootKeyHints = []key.Binding{refreshKey, tabNumberKey, nextTabKey, quitKey}

// tasksSourceName names the task source in load errors.
const tasksSourceName = "tasks"

// dataSource is one kind of data the root loads for the Views. load returns
// the message for the Views and the error the load failed with.
type dataSource struct {
	name string
	load func(projectSlug string) (tea.Msg, error)
}

// loadKey names the data of one source for one project.
type loadKey struct {
	projectSlug string
	sourceName  string
}

// refreshTickMsg starts the loads of a refresh.
type refreshTickMsg struct{}

// dataLoadedMsg carries the result of one load to the root.
type dataLoadedMsg struct {
	key     loadKey
	viewMsg tea.Msg
	err     error
}

// Model is the root of the screen: a top bar, the active Tab and a bottom bar.
// It loads the data on every refresh and sends it to the Views of every Tab.
type Model struct {
	project         *project.Project
	source          TaskSource
	styles          *styles.Styles
	warnings        []string
	tabs            []*Tab
	activeTabIndex  int
	refreshInterval time.Duration
	runningLoads    map[loadKey]bool
	loadErrors      map[loadKey]error
	help            help.Model
	width           int
	height          int
}

// NewModel returns the screen of a project with the first Tab active. It
// reloads the data every refresh interval, shows the warnings in the bottom
// bar and refuses an empty list of Tabs.
func NewModel(shownProject *project.Project, source TaskSource, screenStyles *styles.Styles, tabs []*Tab, refreshInterval time.Duration, warnings []string) (*Model, error) {
	if len(tabs) == 0 {
		return nil, errors.New("the TUI needs at least one tab to show")
	}
	keyHelp := help.New()
	keyHelp.Styles = screenStyles.Help
	return &Model{
		project:         shownProject,
		source:          source,
		styles:          screenStyles,
		warnings:        warnings,
		tabs:            tabs,
		refreshInterval: refreshInterval,
		runningLoads:    map[loadKey]bool{},
		loadErrors:      map[loadKey]error{},
		help:            keyHelp,
	}, nil
}

// Init loads the data and starts the refresh ticks.
func (model *Model) Init() tea.Cmd {
	return tea.Batch(model.refresh(), model.nextTick())
}

func (model *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.handleResize(msg)
		return model, nil
	case tea.KeyPressMsg:
		return model, model.handleKey(msg)
	case refreshTickMsg:
		return model, tea.Batch(model.refresh(), model.nextTick())
	case dataLoadedMsg:
		return model, model.handleDataLoaded(msg)
	}
	return model, nil
}

func (model *Model) nextTick() tea.Cmd {
	return tea.Tick(model.refreshInterval, func(time.Time) tea.Msg { return refreshTickMsg{} })
}

// dataSources returns every source the root loads for the project.
func (model *Model) dataSources() []dataSource {
	return []dataSource{{name: tasksSourceName, load: model.loadTasks}}
}

// refresh starts one load of every source of the project. It skips a source
// whose last load still runs.
func (model *Model) refresh() tea.Cmd {
	var commands []tea.Cmd
	for _, source := range model.dataSources() {
		dataKey := loadKey{projectSlug: model.project.Slug, sourceName: source.name}
		if model.runningLoads[dataKey] {
			continue
		}
		model.runningLoads[dataKey] = true
		commands = append(commands, func() tea.Msg {
			viewMsg, err := source.load(dataKey.projectSlug)
			return dataLoadedMsg{key: dataKey, viewMsg: viewMsg, err: err}
		})
	}
	return tea.Batch(commands...)
}

func (model *Model) loadTasks(projectSlug string) (tea.Msg, error) {
	listed, err := model.source.ListTasks(projectSlug, task.ListTasksFilter{}, firstPage, allOnOnePage)
	return views.TasksLoadedMsg{ProjectSlug: projectSlug, Tasks: listed.Items, Err: err}, err
}

// handleDataLoaded ends a load, keeps its error for the bottom bar and sends
// its message to the Views of every Tab.
func (model *Model) handleDataLoaded(msg dataLoadedMsg) tea.Cmd {
	delete(model.runningLoads, msg.key)
	if msg.err != nil {
		model.loadErrors[msg.key] = msg.err
	} else {
		delete(model.loadErrors, msg.key)
	}
	return model.broadcast(msg.viewMsg)
}

func (model *Model) handleResize(msg tea.WindowSizeMsg) {
	model.width = msg.Width
	model.height = msg.Height
	model.help.SetWidth(msg.Width)
	for _, tab := range model.tabs {
		tab.SetSize(msg.Width, max(msg.Height-barHeight, 0))
	}
}

// handleKey quits, refreshes, switches Tabs or passes the key to the focused
// View of the active Tab. A number key with no Tab behind it does nothing.
func (model *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, quitKey):
		return tea.Quit
	case key.Matches(msg, refreshKey):
		return model.refresh()
	case key.Matches(msg, nextTabKey):
		model.nextTab()
	case key.Matches(msg, previousTabKey):
		model.previousTab()
	case key.Matches(msg, tabNumberKey):
		tabNumber, _ := strconv.Atoi(msg.String())
		if tabNumber <= len(model.tabs) {
			model.activeTabIndex = tabNumber - 1
		}
	default:
		return model.ActiveTab().FocusedTile().view.Update(msg)
	}
	return nil
}

// nextTab makes the next Tab active. The last Tab wraps to the first.
func (model *Model) nextTab() {
	model.activeTabIndex++
	if model.activeTabIndex == len(model.tabs) {
		model.activeTabIndex = 0
	}
}

// previousTab makes the previous Tab active. The first Tab wraps to the last.
func (model *Model) previousTab() {
	if model.activeTabIndex == 0 {
		model.activeTabIndex = len(model.tabs)
	}
	model.activeTabIndex--
}

// broadcast sends a message to the Views of every Tab.
func (model *Model) broadcast(msg tea.Msg) tea.Cmd {
	commands := make([]tea.Cmd, 0, len(model.tabs))
	for _, tab := range model.tabs {
		commands = append(commands, tab.Broadcast(msg))
	}
	return tea.Batch(commands...)
}

// ActiveTab returns the Tab the screen shows.
func (model *Model) ActiveTab() *Tab {
	return model.tabs[model.activeTabIndex]
}

func (model *Model) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true
	view.WindowTitle = fmt.Sprintf(windowTitleFormat, model.project.Name)
	if model.width <= 0 || model.height <= 0 {
		return view
	}
	lines := []string{model.renderTopBar()}
	if model.height > barHeight {
		lines = append(lines, model.renderActiveTab())
	}
	if model.height > 1 {
		lines = append(lines, model.renderBottomBar())
	}
	view.Content = strings.Join(lines, "\n")
	return view
}

// renderActiveTab returns the lines of the active Tab, or a warning in their
// place when the Tab has no room for its Tiles.
func (model *Model) renderActiveTab() string {
	activeTab := model.ActiveTab()
	if activeTab.HasRoomForTiles() {
		return activeTab.Render()
	}
	warning := cells.Fit(model.renderWarning(fmt.Sprintf(tooSmallFormat, activeTab.Title)), model.width)
	linesBelowWarning := model.height - barHeight - 1
	if linesBelowWarning == 0 {
		return warning
	}
	return warning + "\n" + cells.BlankLines(model.width, linesBelowWarning)
}

// renderTopBar shows the Tabs.
func (model *Model) renderTopBar() string {
	return cells.FitEllipsis(model.renderTabBar(model.width), model.width)
}

// renderTabBar returns the Tab labels. When they are wider than the space, it
// drops Tabs from the left until the active Tab fits.
func (model *Model) renderTabBar(width int) string {
	labels := make([]string, len(model.tabs))
	for index, tab := range model.tabs {
		labelStyle := model.styles.Tab
		if index == model.activeTabIndex {
			labelStyle = model.styles.ActiveTab
		}
		labels[index] = labelStyle.Render(renderTabLabel(index, tab.Title))
	}
	firstShownIndex := model.firstShownTabIndex(labels, width)
	return strings.Join(labels[firstShownIndex:], "")
}

// firstShownTabIndex returns the index of the leftmost Tab the tab bar shows.
// It hides Tabs from the left until the labels up to the active Tab fit in the
// space. The active Tab is never hidden.
func (model *Model) firstShownTabIndex(labels []string, width int) int {
	firstShownIndex := 0
	for firstShownIndex < model.activeTabIndex && isWider(labels[firstShownIndex:model.activeTabIndex+1], width) {
		firstShownIndex++
	}
	return firstShownIndex
}

// isWider reports whether the labels side by side take more cells than the
// space.
func isWider(labels []string, width int) bool {
	return ansi.StringWidth(strings.Join(labels, "")) > width
}

func renderTabLabel(index int, title string) string {
	tabNumber := index + 1
	if tabNumber <= numberedTabCount {
		return fmt.Sprintf(numberedTabLabelFormat, tabNumber, title)
	}
	return fmt.Sprintf(tabLabelFormat, title)
}

// renderBottomBar shows the status segments of the active Tab, the load
// errors and the warnings on the left and the key hints on the right.
func (model *Model) renderBottomBar() string {
	bindings := slices.Concat(model.ActiveTab().FocusedTile().view.KeyHints(), rootKeyHints)
	hints := model.help.ShortHelpView(bindings)
	notices := model.renderLoadErrors()
	for _, warning := range model.warnings {
		notices = append(notices, model.renderWarning(warning))
	}
	statuses := ""
	if activeStatuses := model.ActiveTab().Statuses(); len(activeStatuses) > 0 {
		statuses = model.styles.Muted.Render(strings.Join(activeStatuses, statusSeparator))
	}
	room := model.width - ansi.StringWidth(hints) - spreadGap
	left := joinStatusLine(statuses, strings.Join(notices, warningSeparator), room)
	return spread(left, hints, model.width)
}

// renderLoadErrors returns the errors of the last loads of the project, in
// the order of its sources.
func (model *Model) renderLoadErrors() []string {
	var rendered []string
	for _, source := range model.dataSources() {
		err, ok := model.loadErrors[loadKey{projectSlug: model.project.Slug, sourceName: source.name}]
		if ok {
			rendered = append(rendered, model.styles.Error.Render(loadErrorPrefix+fmt.Sprintf(loadErrorFormat, source.name, err)))
		}
	}
	return rendered
}

func (model *Model) renderWarning(warning string) string {
	return model.styles.Warning.Render(warningPrefix + warning)
}

// joinStatusLine puts the notices after the status segments within room
// cells. It cuts the status segments from the end first and drops them when
// the notices alone do not fit. It leaves the notices whole for the bar to
// cut.
func joinStatusLine(statuses, notices string, room int) string {
	switch {
	case statuses == "":
		return notices
	case notices == "":
		return statuses
	}
	statusRoom := room - ansi.StringWidth(notices) - len(warningSeparator)
	if statusRoom <= 0 {
		return notices
	}
	return ansi.Truncate(statuses, statusRoom, cells.Ellipsis) + warningSeparator + notices
}

// spread lays out one line with one text at its start and another at its end.
// It cuts the start text to keep the end text whole and a gap between them. It
// cuts the end text when it is wider than the line.
func spread(left, right string, width int) string {
	right = ansi.Truncate(right, width, "")
	room := width - ansi.StringWidth(right)
	if room <= spreadGap {
		return strings.Repeat(" ", room) + right
	}
	left = ansi.Truncate(left, room-spreadGap, cells.Ellipsis)
	return cells.Fit(left, room) + right
}
