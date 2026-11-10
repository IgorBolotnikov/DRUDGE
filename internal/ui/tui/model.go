package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// overlayZ puts an overlay above the screen.
const overlayZ = 1

// tabTop is the line of the screen the Tab starts on, below the top bar.
const tabTop = 1

// barHeight is how many lines the top bar and the bottom bar take together.
const barHeight = 2

// topBarHeight is how many lines the top bar takes above the active Tab.
const topBarHeight = 1

// warningPrefix starts a warning in the bottom bar.
const warningPrefix = "! "

// tooSmallFormat is the warning in place of a Tab that has no room for its
// Tiles.
const tooSmallFormat = "terminal too small for tab %q, zoom with z or resize"

// loadErrorPrefix starts a load error in the bottom bar.
const loadErrorPrefix = "✗ "

// loadErrorFormat is the load error of one source in the bottom bar.
const loadErrorFormat = "could not load %s: %v"

// checkErrorFormat is the error of the last Drudger check in the bottom bar.
const checkErrorFormat = "could not check Drudgers: %v"

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

// keyTaker is a part of the screen that may take every key but ctrl+c.
type keyTaker interface {
	TakesAllKeys() bool
	Update(msg tea.Msg) tea.Cmd
}

// Names of the data sources in load errors.
const (
	tasksSourceName    = "tasks"
	drudgersSourceName = "Drudgers"
)

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

// drudgerCheckTickMsg starts a check of the Drudgers.
type drudgerCheckTickMsg struct{}

// drudgersCheckedMsg carries the end of a Drudger check to the root.
type drudgersCheckedMsg struct {
	viewMsg views.DrudgersCheckedMsg
}

// Intervals are how long the root waits between two loads of its data and
// between two checks of the Drudgers.
type Intervals struct {
	Refresh      time.Duration
	DrudgerCheck time.Duration
}

// dataLoadedMsg carries the result of one load to the root.
type dataLoadedMsg struct {
	key     loadKey
	viewMsg tea.Msg
	err     error
}

// Model is the root of the screen: a top bar, the active Tab and a bottom bar.
// It loads the data on every refresh, checks the Drudgers on every Drudger
// check and sends both to the Views of every Tab and of the task overlay.
type Model struct {
	project         *project.Project
	sources         Sources
	styles          *styles.Styles
	warnings        []string
	tabs            []*Tab
	dataSources     []dataSource
	activeTabIndex  int
	intervals       Intervals
	runningLoads    map[loadKey]bool
	loadErrors      map[loadKey]error
	isChecking      bool
	checkErr        error
	isMouseCaptured bool
	help            help.Model
	helpOverlay     helpOverlay
	taskOverlay     taskOverlay
	width           int
	height          int
}

// NewModel returns the screen of a project with the first Tab active. It
// reloads the data and checks the Drudgers at the intervals, shows the
// warnings in the bottom bar, takes the mouse from the terminal when it is
// captured and refuses an empty list of Tabs.
func NewModel(shownProject *project.Project, sources Sources, screenStyles *styles.Styles, tabs []*Tab, intervals Intervals, isMouseCaptured bool, warnings []string) (*Model, error) {
	if len(tabs) == 0 {
		return nil, errors.New("the TUI needs at least one tab to show")
	}
	keyHelp := help.New()
	keyHelp.Styles = screenStyles.Help
	model := &Model{
		project:         shownProject,
		sources:         sources,
		styles:          screenStyles,
		warnings:        warnings,
		tabs:            tabs,
		intervals:       intervals,
		runningLoads:    map[loadKey]bool{},
		loadErrors:      map[loadKey]error{},
		help:            keyHelp,
		isMouseCaptured: isMouseCaptured,
		taskOverlay:     newTaskOverlay(screenStyles),
	}
	model.dataSources = []dataSource{
		{name: tasksSourceName, load: model.loadTasks},
		{name: drudgersSourceName, load: model.loadDrudgers},
	}
	return model, nil
}

// Init loads the data, checks the Drudgers and starts the ticks of both.
func (model *Model) Init() tea.Cmd {
	return tea.Batch(model.refresh(), model.nextTick(), model.checkDrudgers(), model.nextCheckTick())
}

func (model *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.handleResize(msg)
		return model, nil
	case tea.KeyPressMsg:
		return model, model.handleKey(msg)
	case tea.MouseClickMsg:
		return model, model.handleClick(msg)
	case tea.MouseWheelMsg:
		return model, model.handleWheel(msg)
	case refreshTickMsg:
		return model, tea.Batch(model.refresh(), model.nextTick())
	case dataLoadedMsg:
		return model, model.handleDataLoaded(msg)
	case drudgerCheckTickMsg:
		return model, tea.Batch(model.checkDrudgers(), model.nextCheckTick())
	case drudgersCheckedMsg:
		return model, model.handleDrudgersChecked(msg)
	case tabMsg:
		return model, model.handleTabMsg(msg)
	}
	return model, nil
}

func (model *Model) nextTick() tea.Cmd {
	return tea.Tick(model.intervals.Refresh, func(time.Time) tea.Msg { return refreshTickMsg{} })
}

func (model *Model) nextCheckTick() tea.Cmd {
	return tea.Tick(model.intervals.DrudgerCheck, func(time.Time) tea.Msg { return drudgerCheckTickMsg{} })
}

// refresh starts one load of every source of the project. It skips a source
// whose last load still runs.
func (model *Model) refresh() tea.Cmd {
	var commands []tea.Cmd
	for _, source := range model.dataSources {
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
	listed, err := model.sources.Tasks.ListTasks(projectSlug, task.ListTasksFilter{}, firstPage, allOnOnePage)
	return views.TasksLoadedMsg{ProjectSlug: projectSlug, Tasks: listed.Items, Err: err}, err
}

func (model *Model) loadDrudgers(projectSlug string) (tea.Msg, error) {
	drudgers, err := model.sources.Drudgers.ReadDrudgers(projectSlug)
	return views.DrudgersLoadedMsg{ProjectSlug: projectSlug, Drudgers: drudgers, ReadAt: time.Now(), Err: err}, err
}

// checkDrudgers starts a check of the Drudgers of the project. It skips the
// check while the last one runs.
func (model *Model) checkDrudgers() tea.Cmd {
	if model.isChecking {
		return nil
	}
	model.isChecking = true
	projectSlug := model.project.Slug
	return func() tea.Msg {
		_, err := model.sources.Drudgers.ListDrudgers(projectSlug, firstPage, allOnOnePage)
		return drudgersCheckedMsg{viewMsg: views.DrudgersCheckedMsg{ProjectSlug: projectSlug, CheckedAt: time.Now(), Err: err}}
	}
}

// handleDrudgersChecked ends a check, keeps its error for the bottom bar and
// sends its message to the Views of every Tab.
func (model *Model) handleDrudgersChecked(msg drudgersCheckedMsg) tea.Cmd {
	model.isChecking = false
	model.checkErr = msg.viewMsg.Err
	return model.broadcast(msg.viewMsg)
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

// handleTabMsg handles a message of a command of a View of a Tab. A selection
// goes to the Tab. A task the Tab has no View to show opens in the task
// overlay. Each command of a batch gets the tag of the Tab, and every other
// message goes back to the program untagged.
func (model *Model) handleTabMsg(msg tabMsg) tea.Cmd {
	switch tabbed := msg.msg.(type) {
	case nil:
		return nil
	case views.TaskSelectedMsg:
		msg.tab.SelectTask(tabbed.TaskID)
		return nil
	case views.OpenTaskMsg:
		if !msg.tab.OpenTask(tabbed.TaskID) {
			model.taskOverlay.open(tabbed.TaskID)
		}
		return nil
	case tea.BatchMsg:
		commands := make([]tea.Cmd, 0, len(tabbed))
		for _, cmd := range tabbed {
			commands = append(commands, msg.tab.tag(cmd))
		}
		return tea.Batch(commands...)
	default:
		return func() tea.Msg { return tabbed }
	}
}

func (model *Model) handleResize(msg tea.WindowSizeMsg) {
	model.width = msg.Width
	model.height = msg.Height
	tabHeight := max(msg.Height-barHeight, 0)
	for _, tab := range model.tabs {
		tab.SetSize(msg.Width, tabHeight)
	}
	model.taskOverlay.SetSize(msg.Width, tabHeight)
}

// handleKey quits, refreshes, checks the Drudgers, opens the help overlay,
// switches Tabs, moves focus between the Tiles of the active Tab, zooms the
// focused Tile or passes the key to the active Tab. A number key with no Tab
// behind it does nothing. The part of the screen that takes all keys gets
// every key but ctrl+c.
func (model *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if key.Matches(msg, interruptKey) {
		return tea.Quit
	}
	if taker, ok := model.allKeysTaker(); ok {
		return taker.Update(msg)
	}
	activeTab := model.ActiveTab()
	switch {
	case key.Matches(msg, quitKey):
		return tea.Quit
	case key.Matches(msg, refreshKey):
		return model.refresh()
	case key.Matches(msg, checkKey):
		return model.checkDrudgers()
	case key.Matches(msg, helpKey):
		model.helpOverlay.open()
	case key.Matches(msg, nextTabKey):
		model.nextTab()
	case key.Matches(msg, previousTabKey):
		model.previousTab()
	case key.Matches(msg, tabNumberKey):
		tabNumber, _ := strconv.Atoi(msg.String())
		if tabNumber <= len(model.tabs) {
			model.activeTabIndex = tabNumber - 1
		}
	case key.Matches(msg, focusLeftKey):
		activeTab.FocusToward(Left)
	case key.Matches(msg, focusDownKey):
		activeTab.FocusToward(Down)
	case key.Matches(msg, focusUpKey):
		activeTab.FocusToward(Up)
	case key.Matches(msg, focusRightKey):
		activeTab.FocusToward(Right)
	case key.Matches(msg, nextTileKey):
		activeTab.FocusNextTile()
	case key.Matches(msg, previousTileKey):
		activeTab.FocusPreviousTile()
	case key.Matches(msg, zoomKey):
		activeTab.ToggleZoom()
	default:
		return activeTab.Update(msg)
	}
	return nil
}

// handleClick switches to the Tab of a label in the top bar, or gives focus to
// the Tile under the pointer and passes the click to its View. It takes only
// the left button and does nothing while an overlay is open.
func (model *Model) handleClick(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Button != tea.MouseLeft || model.isOverlayOpen() {
		return nil
	}
	if msg.Y < topBarHeight {
		if tabIndex, ok := model.tabAt(msg.X); ok {
			model.activeTabIndex = tabIndex
		}
		return nil
	}
	inTab, ok := model.tabMouse(tea.Mouse(msg))
	if !ok {
		return nil
	}
	return model.ActiveTab().Click(tea.MouseClickMsg(inTab))
}

// handleWheel passes a turn of the wheel to the View of the Tile under the
// pointer, focused or not. It does nothing while an overlay is open.
func (model *Model) handleWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if model.isOverlayOpen() {
		return nil
	}
	inTab, ok := model.tabMouse(tea.Mouse(msg))
	if !ok {
		return nil
	}
	return model.ActiveTab().Wheel(tea.MouseWheelMsg(inTab))
}

func (model *Model) isOverlayOpen() bool {
	return model.helpOverlay.isOpen || model.taskOverlay.isOpen
}

// tabMouse moves a mouse event on the screen into the cells of the active Tab.
// ok is false outside the Tab and while the Tab shows a warning in place of
// its Tiles.
func (model *Model) tabMouse(mouse tea.Mouse) (moved tea.Mouse, ok bool) {
	tabHeight := model.height - barHeight
	mouse.Y -= topBarHeight
	isInTab := mouse.Y >= 0 && mouse.Y < tabHeight
	if !isInTab || !model.ActiveTab().HasRoomForTiles() {
		return tea.Mouse{}, false
	}
	return mouse, true
}

// tabAt returns the index of the Tab whose label the top bar shows at a
// column. ok is false for a column with no label.
func (model *Model) tabAt(x int) (tabIndex int, ok bool) {
	for _, label := range model.tabBarLabels(model.width) {
		if x >= label.x && x < label.x+label.width {
			return label.tabIndex, true
		}
	}
	return 0, false
}

// allKeysTaker returns the part of the screen that takes all keys: the open
// help overlay, the open task overlay, then the active Tab. ok is false when
// none does.
func (model *Model) allKeysTaker() (taker keyTaker, ok bool) {
	for _, candidate := range []keyTaker{&model.helpOverlay, &model.taskOverlay, model.ActiveTab()} {
		if candidate.TakesAllKeys() {
			return candidate, true
		}
	}
	return nil, false
}

// keyMap returns the keys of the focused View and the root keys.
func (model *Model) keyMap() keyMap {
	kind, _ := findViewKind(model.ActiveTab().FocusedTile().view.Name())
	return keyMap{viewKeys: kind.keys}
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

// broadcast sends a message to the Views of every Tab and to the task
// overlay.
func (model *Model) broadcast(msg tea.Msg) tea.Cmd {
	commands := make([]tea.Cmd, 0, len(model.tabs)+1)
	for _, tab := range model.tabs {
		commands = append(commands, tab.Broadcast(msg))
	}
	commands = append(commands, model.taskOverlay.Update(msg))
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
	if model.isMouseCaptured {
		view.MouseMode = tea.MouseModeCellMotion
	}
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
	if model.taskOverlay.isOpen && model.height > barHeight {
		view.Content = model.drawOver(view.Content, model.taskOverlay.tile.Render(), taskOverlayMarginX, tabTop+taskOverlayMarginY)
	}
	if model.helpOverlay.isOpen {
		view.Content = model.renderHelpOverlay(view.Content)
	}
	return view
}

// renderHelpOverlay draws the help box centered over the screen. A box bigger
// than the screen starts at the top left corner.
func (model *Model) renderHelpOverlay(screen string) string {
	box := model.styles.HelpBox.Render(model.help.FullHelpView(model.keyMap().FullHelp()))
	boxX := max((model.width-lipgloss.Width(box))/2, 0)
	boxY := max((model.height-lipgloss.Height(box))/2, 0)
	return model.drawOver(screen, box, boxX, boxY)
}

// drawOver draws a box over the screen with its top left corner at x and y.
// The screen under the box keeps its layout. A box that reaches past the
// screen is cut at the right and the bottom edge.
func (model *Model) drawOver(screen, box string, x, y int) string {
	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(screen),
		lipgloss.NewLayer(box).X(x).Y(y).Z(overlayZ),
	)
	return lipgloss.NewCanvas(model.width, model.height).Compose(compositor).Render()
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

// renderTabBar returns the Tab labels the tab bar shows.
func (model *Model) renderTabBar(width int) string {
	var rendered strings.Builder
	for _, label := range model.tabBarLabels(width) {
		rendered.WriteString(label.rendered)
	}
	return rendered.String()
}

// tabBarLabel is the label of one Tab the tab bar shows.
type tabBarLabel struct {
	tabIndex int
	rendered string
	// x is the column the label starts at.
	x     int
	width int
}

// tabBarLabels returns the labels the tab bar shows in width cells, left to
// right. When the labels of every Tab are wider than the space, it drops Tabs
// from the left until the active Tab fits.
func (model *Model) tabBarLabels(width int) []tabBarLabel {
	labels := make([]string, len(model.tabs))
	for index, tab := range model.tabs {
		labelStyle := model.styles.Tab
		if index == model.activeTabIndex {
			labelStyle = model.styles.ActiveTab
		}
		labels[index] = labelStyle.Render(renderTabLabel(index, tab.Title))
	}
	firstShownIndex := model.firstShownTabIndex(labels, width)
	shown := make([]tabBarLabel, 0, len(labels)-firstShownIndex)
	x := 0
	for index := firstShownIndex; index < len(labels); index++ {
		labelWidth := ansi.StringWidth(labels[index])
		shown = append(shown, tabBarLabel{tabIndex: index, rendered: labels[index], x: x, width: labelWidth})
		x += labelWidth
	}
	return shown
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
// errors and the warnings on the left and the help hint on the right. The hint
// is left out while the focused View takes all keys, since the help key goes
// to the View then. While the task overlay is open, the bar shows the segment
// of the overlay in place of the segments of the Tab and the key that closes
// the overlay as the hint.
func (model *Model) renderBottomBar() string {
	hints := ""
	shownStatuses := model.ActiveTab().Statuses()
	switch {
	case model.taskOverlay.isOpen:
		hints = model.help.ShortHelpView([]key.Binding{closeTaskKey})
		shownStatuses = nil
		if status := model.taskOverlay.tile.Status(); status != "" {
			shownStatuses = []string{status}
		}
	case !model.ActiveTab().TakesAllKeys():
		hints = model.help.ShortHelpView(model.keyMap().ShortHelp())
	}
	notices := model.renderLoadErrors()
	for _, warning := range model.warnings {
		notices = append(notices, model.renderWarning(warning))
	}
	statuses := ""
	if len(shownStatuses) > 0 {
		statuses = model.styles.Muted.Render(strings.Join(shownStatuses, statusSeparator))
	}
	room := model.width - ansi.StringWidth(hints) - spreadGap
	left := joinStatusLine(statuses, strings.Join(notices, warningSeparator), room)
	return spread(left, hints, model.width)
}

// renderLoadErrors returns the errors of the last loads of the project, in
// the order of its sources, and the error of the last Drudger check.
func (model *Model) renderLoadErrors() []string {
	var rendered []string
	for _, source := range model.dataSources {
		err, ok := model.loadErrors[loadKey{projectSlug: model.project.Slug, sourceName: source.name}]
		if ok {
			rendered = append(rendered, model.renderLoadError(fmt.Sprintf(loadErrorFormat, source.name, err)))
		}
	}
	if model.checkErr != nil {
		rendered = append(rendered, model.renderLoadError(fmt.Sprintf(checkErrorFormat, model.checkErr)))
	}
	return rendered
}

func (model *Model) renderLoadError(message string) string {
	return model.styles.Error.Render(loadErrorPrefix + message)
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
