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
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

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

// feedErrorPrefix starts the error of a feed in the bottom bar.
const feedErrorPrefix = "✗ "

// warningSeparator joins the feed errors and the warnings of the bottom bar.
const warningSeparator = "  "

// spreadGap is how many spaces a bar keeps between its left and its right
// part.
const spreadGap = 1

const windowTitleFormat = "DRUDGE | %s"

// Labels of a Tab in the top bar. The first nine Tabs show their number key.
const (
	numberedTabLabelFormat = " %d %s "
	tabLabelFormat         = " %s "
	numberedTabCount       = 9
)

// Model is the root of the screen: a top bar, the active Tab, a bottom bar and
// a stack of overlays over them. It runs the feeds the visible Views show and
// sends the message of each feed to the Views of every Tab and every overlay
// that show the feed.
type Model struct {
	project         *project.Project
	styles          *styles.Styles
	warnings        []string
	tabs            []*Tab
	activeTabIndex  int
	feedRunner      *feeds.Runner
	isMouseCaptured bool
	help            help.Model
	overlays        []Overlay
	width           int
	height          int
}

// NewModel returns the screen of a project with the first Tab active. It runs
// each feed at its interval, shows the warnings in the bottom bar, takes the
// mouse from the terminal when it is captured and refuses an empty list of
// Tabs.
func NewModel(shownProject *project.Project, sources feeds.Sources, screenStyles *styles.Styles, tabs []*Tab, intervals map[views.FeedName]time.Duration, isMouseCaptured bool, warnings []string) (*Model, error) {
	if len(tabs) == 0 {
		return nil, errors.New("the TUI needs at least one tab to show")
	}
	keyHelp := help.New()
	keyHelp.Styles = screenStyles.Help
	return &Model{
		project:         shownProject,
		styles:          screenStyles,
		warnings:        warnings,
		tabs:            tabs,
		feedRunner:      feeds.NewRunner(sources, shownProject.Slug, feedKinds, intervals),
		help:            keyHelp,
		isMouseCaptured: isMouseCaptured,
	}, nil
}

// Init starts the feeds of the visible Views.
func (model *Model) Init() tea.Cmd {
	return model.syncFeeds()
}

// Update handles a message and then starts the feeds the visible Views show
// now and stops the ones they no longer show.
func (model *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := model.handleMsg(msg)
	return model, tea.Batch(cmd, model.syncFeeds())
}

func (model *Model) handleMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.handleResize(msg)
	case tea.KeyPressMsg:
		return model.handleKey(msg)
	case tea.MouseMsg:
		return model.handleMouse(msg)
	case feeds.TickMsg:
		return model.feedRunner.HandleTick(msg)
	case feeds.DoneMsg:
		return model.deliver(model.feedRunner.HandleDone(msg))
	case tabMsg:
		return model.handleTabMsg(msg)
	case overlayMsg:
		return model.handleOverlayMsg(msg)
	}
	return nil
}

// syncFeeds runs the feeds the visible Views show: the Tiles of the active
// Tab and the Views of every overlay. The active Tab stays visible around
// the margins of an overlay.
func (model *Model) syncFeeds() tea.Cmd {
	wanted := model.ActiveTab().Feeds()
	for _, overlay := range model.overlays {
		wanted = append(wanted, overlay.Feeds()...)
	}
	return model.feedRunner.Sync(wanted)
}

// deliver sends the message of a feed to the Views of every Tab and every
// overlay that show the feed. A nil message reaches no View.
func (model *Model) deliver(feed views.Feed, msg tea.Msg) tea.Cmd {
	if msg == nil {
		return nil
	}
	commands := make([]tea.Cmd, 0, len(model.tabs)+len(model.overlays))
	for _, tab := range model.tabs {
		commands = append(commands, tab.Deliver(feed, msg))
	}
	for _, overlay := range model.overlays {
		if slices.Contains(overlay.Feeds(), feed) {
			commands = append(commands, tagOverlay(overlay.Update(msg)))
		}
	}
	return tea.Batch(commands...)
}

// handleTabMsg handles a message of a command of a View of a Tab. A selection
// goes to the Tab. An open message goes to the Tab, and opens in an overlay
// when no View of the Tab shows it. Each command of a batch gets the tag of the
// Tab, and every other message goes back to the program untagged.
func (model *Model) handleTabMsg(msg tabMsg) tea.Cmd {
	switch tabbed := msg.msg.(type) {
	case nil:
		return nil
	case views.TaskSelectedMsg:
		msg.tab.SelectTask(tabbed.TaskID)
		return nil
	case views.OpenMsg:
		if !msg.tab.Open(tabbed) {
			return model.openInOverlay(tabbed)
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

// handleOverlayMsg handles a message of a command of an overlay. An open
// message opens in a new overlay and a selection is dropped. Each command of a
// batch gets the tag of the overlay, and every other message goes back to the
// program untagged.
func (model *Model) handleOverlayMsg(msg overlayMsg) tea.Cmd {
	switch inner := msg.msg.(type) {
	case nil, views.TaskSelectedMsg:
		return nil
	case views.OpenMsg:
		return model.openInOverlay(inner)
	case tea.BatchMsg:
		commands := make([]tea.Cmd, 0, len(inner))
		for _, cmd := range inner {
			commands = append(commands, tagOverlay(cmd))
		}
		return tea.Batch(commands...)
	default:
		return func() tea.Msg { return inner }
	}
}

// openInOverlay builds every View of the registry in order with the default
// values of its options and pushes the first one that shows the open message
// in an overlay. The View gets the last messages of its feeds before it is
// pushed. Nothing opens when no View shows the message.
func (model *Model) openInOverlay(msg views.OpenMsg) tea.Cmd {
	for _, kind := range viewKinds {
		opener, ok := kind.newView(model.styles, kind.defaultOptionValues()).(views.Opener)
		if !ok || !opener.Open(msg) {
			continue
		}
		overlay := newViewOverlay(opener, model.styles)
		lastMessages := model.feedRunner.LastMessages(overlay.Feeds())
		commands := make([]tea.Cmd, 0, len(lastMessages))
		for _, lastMsg := range lastMessages {
			commands = append(commands, tagOverlay(overlay.Update(lastMsg)))
		}
		model.push(overlay)
		return tea.Batch(commands...)
	}
	return nil
}

// push sizes an overlay to the screen and puts it on top of the stack.
func (model *Model) push(overlay Overlay) {
	overlay.SetSize(model.width, model.height)
	model.overlays = append(model.overlays, overlay)
}

// pop takes the overlay on top off the stack.
func (model *Model) pop() {
	if len(model.overlays) > 0 {
		model.overlays = model.overlays[:len(model.overlays)-1]
	}
}

// topOverlay returns the overlay on top of the stack. ok is false while no
// overlay is open.
func (model *Model) topOverlay() (top Overlay, ok bool) {
	if len(model.overlays) == 0 {
		return nil, false
	}
	return model.overlays[len(model.overlays)-1], true
}

func (model *Model) handleResize(msg tea.WindowSizeMsg) {
	model.width = msg.Width
	model.height = msg.Height
	tabHeight := max(msg.Height-barHeight, 0)
	for _, tab := range model.tabs {
		tab.SetSize(msg.Width, tabHeight)
	}
	for _, overlay := range model.overlays {
		overlay.SetSize(msg.Width, msg.Height)
	}
}

// handleKey quits on ctrl+c and passes every other key to the overlay on top
// while one is open. With no overlay open it opens the help, switches Tabs, moves focus between the Tiles of the active
// Tab, zooms the focused Tile or passes the key to the active Tab. A number key
// with no Tab behind it does nothing. A Tab that takes all keys gets every key
// but ctrl+c.
func (model *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if key.Matches(msg, interruptKey) {
		return tea.Quit
	}
	if top, ok := model.topOverlay(); ok {
		return model.handleOverlayKey(top, msg)
	}
	activeTab := model.ActiveTab()
	if activeTab.TakesAllKeys() {
		return activeTab.Update(msg)
	}
	switch {
	case key.Matches(msg, quitKey):
		return tea.Quit
	case key.Matches(msg, helpKey):
		model.pushHelp()
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

// handleOverlayKey passes a key to the overlay on top when it takes all keys.
// Otherwise esc closes the overlay, the help key closes the help or opens it
// over the overlay, and every other key goes to the overlay.
func (model *Model) handleOverlayKey(top Overlay, msg tea.KeyPressMsg) tea.Cmd {
	_, isHelp := top.(*helpOverlay)
	switch {
	case top.TakesAllKeys():
		return tagOverlay(top.Update(msg))
	case key.Matches(msg, closeOverlayKey):
		model.pop()
	case key.Matches(msg, helpKey) && isHelp:
		model.pop()
	case key.Matches(msg, helpKey):
		model.pushHelp()
	default:
		return tagOverlay(top.Update(msg))
	}
	return nil
}

// pushHelp opens the help with the keys that work on the screen right now.
func (model *Model) pushHelp() {
	model.push(newHelpOverlay(model.help, model.styles, model.helpSections()))
}

// helpSections returns the keys the help lists, each section titled by what
// the keys act on. With no overlay open they are the keys of the focused View
// under the title of its Tile and the root keys. With an overlay on top they
// are the keys of the overlay and the keys that close it and open the help.
func (model *Model) helpSections() []helpSection {
	if top, ok := model.topOverlay(); ok {
		return []helpSection{
			{title: top.Title(), keys: top.Keys()},
			{title: globalHelpTitle, keys: overlayBindings},
		}
	}
	focusedTile := model.ActiveTab().FocusedTile()
	kind, _ := findViewKind(focusedTile.view.Name())
	return []helpSection{
		{title: focusedTile.Title(), keys: kind.keys},
		{title: globalHelpTitle, keys: rootBindings},
	}
}

// handleMouse takes only left clicks and turns of the wheel. It passes them to
// the overlay on top while one is open. Otherwise a click on a label in the top
// bar switches to its Tab, and every other event goes to the active Tab.
func (model *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if !isLeftClickOrWheel(msg) {
		return nil
	}
	if top, ok := model.topOverlay(); ok {
		return tagOverlay(top.Mouse(msg))
	}
	mouse := msg.Mouse()
	if _, isClick := msg.(tea.MouseClickMsg); isClick && mouse.Y < topBarHeight {
		if tabIndex, ok := model.tabAt(mouse.X); ok {
			model.activeTabIndex = tabIndex
		}
		return nil
	}
	inTab, ok := model.tabMouse(msg)
	if !ok {
		return nil
	}
	return model.ActiveTab().Mouse(inTab)
}

// isLeftClickOrWheel reports whether a mouse event is a click of the left
// button or a turn of the wheel.
func isLeftClickOrWheel(msg tea.MouseMsg) bool {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		return msg.Button == tea.MouseLeft
	case tea.MouseWheelMsg:
		return true
	}
	return false
}

// tabMouse moves a mouse event on the screen into the cells of the active Tab.
// ok is false outside the Tab and while the Tab shows a warning in place of
// its Tiles.
func (model *Model) tabMouse(msg tea.MouseMsg) (moved tea.MouseMsg, ok bool) {
	tabHeight := model.height - barHeight
	inTabY := msg.Mouse().Y - topBarHeight
	isInTab := inTabY >= 0 && inTabY < tabHeight
	if !isInTab || !model.ActiveTab().HasRoomForTiles() {
		return nil, false
	}
	return moveMouse(msg, 0, -topBarHeight), true
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
	_, isOverlayOpen := model.topOverlay()
	model.ActiveTab().SetFocusHidden(isOverlayOpen)
	if model.height > barHeight {
		lines = append(lines, model.renderActiveTab())
	}
	if model.height > 1 {
		lines = append(lines, model.renderBottomBar())
	}
	view.Content = model.drawOverlays(strings.Join(lines, "\n"))
	return view
}

// drawOverlays draws every overlay over the screen, each one above the one
// below it in the stack. The screen under an overlay keeps its layout. A box
// that reaches past the screen is cut at the right and the bottom edge.
func (model *Model) drawOverlays(screen string) string {
	if len(model.overlays) == 0 {
		return screen
	}
	layers := []*lipgloss.Layer{lipgloss.NewLayer(screen)}
	for index, overlay := range model.overlays {
		box, x, y := overlay.Render()
		if box != "" {
			layers = append(layers, lipgloss.NewLayer(box).X(x).Y(y).Z(index+1))
		}
	}
	compositor := lipgloss.NewCompositor(layers...)
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

// renderBottomBar shows the feed errors and the warnings on the left and the
// help hint on the right. While an overlay is open, the hint is the hint of the
// overlay on top. The hint is left out while the focused View or the overlay
// on top takes all keys, since the keys of the hint go to it then.
func (model *Model) renderBottomBar() string {
	shortHelp := []key.Binding{helpKey}
	isTakingAllKeys := model.ActiveTab().TakesAllKeys()
	if top, ok := model.topOverlay(); ok {
		shortHelp = top.ShortHelp()
		isTakingAllKeys = top.TakesAllKeys()
	}
	hints := ""
	if !isTakingAllKeys {
		hints = model.help.ShortHelpView(shortHelp)
	}
	notices := model.renderFeedErrors()
	for _, warning := range model.warnings {
		notices = append(notices, model.renderWarning(warning))
	}
	return spread(strings.Join(notices, warningSeparator), hints, model.width)
}

// renderFeedErrors returns the error of every running feed whose last run
// failed.
func (model *Model) renderFeedErrors() []string {
	errs := model.feedRunner.Errors()
	rendered := make([]string, 0, len(errs))
	for _, err := range errs {
		rendered = append(rendered, model.styles.Error.Render(feedErrorPrefix+err.Error()))
	}
	return rendered
}

func (model *Model) renderWarning(warning string) string {
	return model.styles.Warning.Render(warningPrefix + warning)
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
