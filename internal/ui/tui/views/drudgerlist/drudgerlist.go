// Package drudgerlist is the View that lists the Drudgers of a project.
package drudgerlist

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/style"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/table"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Name is the kind of the View that lists Drudgers.
const Name = "drudger-list"

// Description says in one line what the View shows.
const Description = "Lists the Drudgers of the project with what DRUDGE last saw of each."

// Titles of the Drudger list columns.
const (
	slotTitle        = "SLOT"
	sandboxTitle     = "SANDBOX"
	agentTitle       = "AGENT"
	taskTitle        = "TASK"
	lastCheckedTitle = "LAST CHECKED"
)

// Indexes of the cells of a row, in the order of the columns.
const (
	slotCell = iota
	sandboxCell
	agentCell
	taskCell
	lastCheckedCell
)

const (
	idleLabel      = "idle"
	neverLabel     = "never"
	cursorMarker   = "▸ "
	noCursorMarker = "  "
)

// Status segments of the Drudger list: the number of the cursor row with the
// Drudger count, and the last check.
const (
	cursorStatusFormat  = "slot %d/%d"
	checkedStatusFormat = "checked %s"
	notCheckedStatus    = "not checked yet"
	statusSeparator     = " · "
)

// headerLines is how many lines sit above the rows.
const headerLines = 1

// wheelScrollLines is how many lines one turn of the wheel scrolls.
const wheelScrollLines = 1

// Formats of how long ago a moment was.
const (
	secondsAgoFormat = "%ds ago"
	minutesAgoFormat = "%dm ago"
	hoursAgoFormat   = "%dh ago"
	daysAgoFormat    = "%dd ago"
)

const day = 24 * time.Hour

// Keys of the Drudger list. The up key and the top key carry the help of
// their pair.
var (
	moveUpKey     = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "move"))
	moveDownKey   = key.NewBinding(key.WithKeys("j", "down"))
	jumpTopKey    = key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g/G", "top/bottom"))
	jumpBottomKey = key.NewBinding(key.WithKeys("G", "end"))
	openTaskKey   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open task"))
	openRunLogKey = key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "runlog"))
)

// Keys are the keys of the Drudger list the help lists.
var Keys = []key.Binding{moveUpKey, jumpTopKey, openTaskKey, openRunLogKey}

// columns of the Drudger list. The slot always stays. The last check drops
// first, then the task, the agent and the sandbox.
var columns = []table.Column{
	{Title: slotTitle, IsRequired: true},
	{Title: sandboxTitle, Priority: 4},
	{Title: agentTitle, Priority: 3},
	{Title: taskTitle, Priority: 2},
	{Title: lastCheckedTitle, Priority: 1},
}

// View lists the Drudgers of a project as the Drudger file records them, with
// a cursor on one row. A failed read keeps the Drudgers of the last read that
// worked. Every "ago" of the View counts from the latest read or check.
type View struct {
	styles      *styles.Styles
	drudgers    []*drudger.Drudger
	hasDrudgers bool
	loadErr     error
	now         time.Time
	checkedAt   time.Time
	layout      table.Layout
	cursor      int
	offset      int
	width       int
	height      int
}

// New returns a Drudger list that shows a loading line until it gets the
// Drudgers.
func New(viewStyles *styles.Styles) *View {
	return &View{styles: viewStyles, layout: table.NewLayout(columns, nil, 0)}
}

func (view *View) Name() string {
	return Name
}

// TakesAllKeys reports false. The Drudger list has no input.
func (view *View) TakesAllKeys() bool {
	return false
}

// Status returns the number of the cursor row with the Drudger count, and how
// long ago the Drudgers were last checked, like slot 2/5 · checked 12s ago.
// With no Drudgers it returns only the last check.
func (view *View) Status() string {
	checkStatus := notCheckedStatus
	if !view.checkedAt.IsZero() {
		checkStatus = fmt.Sprintf(checkedStatusFormat, formatAgo(view.checkedAt, view.now))
	}
	if len(view.drudgers) == 0 {
		return checkStatus
	}
	cursorNumber := view.cursor + 1
	return fmt.Sprintf(cursorStatusFormat, cursorNumber, len(view.drudgers)) + statusSeparator + checkStatus
}

// Feeds returns the Drudgers feed and the Drudger check feed.
func (view *View) Feeds() []views.Feed {
	return []views.Feed{{Name: views.DrudgersFeed}, {Name: views.DrudgerCheckFeed}}
}

func (view *View) SetSize(width, height int) {
	view.width = width
	view.height = height
	view.measureColumns()
	view.scrollToCursor()
}

// Update takes the read Drudgers and the end of a check, moves the cursor on
// keys, moves the cursor to a clicked row and scrolls on the wheel. Moving the
// cursor onto another busy Drudger selects its task. Enter opens the task of
// the Drudger under the cursor and L opens its runlog.
func (view *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case views.DrudgersLoadedMsg:
		view.loadErr = msg.Err
		if msg.Err == nil {
			view.now = later(view.now, msg.ReadAt)
			view.showDrudgers(msg.Drudgers)
		}
	case views.DrudgersCheckedMsg:
		if msg.Err == nil {
			view.checkedAt = msg.CheckedAt
			view.now = later(view.now, msg.CheckedAt)
			view.measureColumns()
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, openTaskKey):
			return view.openTask()
		case key.Matches(msg, openRunLogKey):
			return view.openRunLog()
		}
		previousCursor := view.cursorDrudger()
		view.moveCursor(msg)
		return view.selectMovedCursor(previousCursor)
	case tea.MouseClickMsg:
		previousCursor := view.cursorDrudger()
		view.handleClick(msg)
		return view.selectMovedCursor(previousCursor)
	case tea.MouseWheelMsg:
		previousCursor := view.cursorDrudger()
		view.handleWheel(msg)
		return view.selectMovedCursor(previousCursor)
	}
	return nil
}

// showDrudgers shows the Drudgers and keeps the cursor on the slot it was on.
// When that slot is gone, the cursor keeps its row or moves up to the last
// row.
func (view *View) showDrudgers(drudgers []*drudger.Drudger) {
	cursorDrudger := view.cursorDrudger()
	view.drudgers = drudgers
	view.hasDrudgers = true
	index := -1
	if cursorDrudger != nil {
		index = slices.IndexFunc(drudgers, func(entry *drudger.Drudger) bool { return entry.Slot == cursorDrudger.Slot })
	}
	if index == -1 {
		index = max(min(view.cursor, len(drudgers)-1), 0)
	}
	view.cursor = index
	view.measureColumns()
	view.scrollToCursor()
}

// handleClick moves the cursor to the row at the line of the click. A click
// on the header or below the last row does nothing.
func (view *View) handleClick(msg tea.MouseClickMsg) {
	rowIndex := view.offset + msg.Y - headerLines
	isOnRow := rowIndex >= view.offset && rowIndex < view.rowsEnd()
	if isOnRow {
		view.cursor = rowIndex
	}
}

// handleWheel scrolls the rows on a turn of the wheel. The cursor moves along
// when its row leaves sight.
func (view *View) handleWheel(msg tea.MouseWheelMsg) {
	switch msg.Button {
	case tea.MouseWheelUp:
		view.scroll(-wheelScrollLines)
	case tea.MouseWheelDown:
		view.scroll(wheelScrollLines)
	}
}

// scroll moves the rows in sight by lines and keeps the cursor on a row in
// sight. It stops at the first and at the last row.
func (view *View) scroll(lines int) {
	lineCount := view.rowLineCount()
	if lineCount <= 0 {
		return
	}
	lastOffset := max(len(view.drudgers)-lineCount, 0)
	view.offset = min(max(view.offset+lines, 0), lastOffset)
	lastShownIndex := view.offset + lineCount - 1
	view.cursor = min(max(view.cursor, view.offset), lastShownIndex)
}

func (view *View) moveCursor(msg tea.KeyPressMsg) {
	last := len(view.drudgers) - 1
	switch {
	case key.Matches(msg, moveUpKey):
		view.cursor = max(view.cursor-1, 0)
	case key.Matches(msg, moveDownKey):
		view.cursor = max(min(view.cursor+1, last), 0)
	case key.Matches(msg, jumpTopKey):
		view.cursor = 0
	case key.Matches(msg, jumpBottomKey):
		view.cursor = max(last, 0)
	}
	view.scrollToCursor()
}

// openTask asks to open the task of the Drudger under the cursor. It does
// nothing when that Drudger is idle or no row is shown.
func (view *View) openTask() tea.Cmd {
	taskID := view.cursorTaskID()
	if taskID == "" {
		return nil
	}
	return func() tea.Msg { return views.OpenTaskMsg{TaskID: taskID} }
}

// openRunLog asks to open the runlog of the task of the Drudger under the
// cursor. It does nothing when that Drudger is idle or no row is shown.
func (view *View) openRunLog() tea.Cmd {
	taskID := view.cursorTaskID()
	if taskID == "" {
		return nil
	}
	return func() tea.Msg { return views.OpenRunLogMsg{TaskID: taskID} }
}

// selectMovedCursor selects the task of the Drudger under the cursor when the
// cursor moved off previousCursor onto a busy Drudger.
func (view *View) selectMovedCursor(previousCursor *drudger.Drudger) tea.Cmd {
	taskID := view.cursorTaskID()
	if taskID == "" || view.cursorDrudger() == previousCursor {
		return nil
	}
	return func() tea.Msg { return views.TaskSelectedMsg{TaskID: taskID} }
}

// cursorDrudger returns the Drudger under the cursor, or nil when no row is
// shown.
func (view *View) cursorDrudger() *drudger.Drudger {
	if view.cursor >= len(view.drudgers) {
		return nil
	}
	return view.drudgers[view.cursor]
}

// cursorTaskID returns the task of the Drudger under the cursor, or an empty
// ID when that Drudger is idle or no row is shown.
func (view *View) cursorTaskID() task.TaskID {
	cursorDrudger := view.cursorDrudger()
	if cursorDrudger == nil {
		return ""
	}
	return cursorDrudger.TaskID
}

// rowLineCount returns how many lines the View has for rows, below the
// header.
func (view *View) rowLineCount() int {
	return view.height - headerLines
}

// rowsEnd returns the index after the last row in sight.
func (view *View) rowsEnd() int {
	return min(view.offset+view.rowLineCount(), len(view.drudgers))
}

// scrollToCursor moves the rows in sight so the cursor row is one of them. It
// scrolls back when rows fit below the last row.
func (view *View) scrollToCursor() {
	lineCount := view.rowLineCount()
	if lineCount <= 0 {
		return
	}
	if view.cursor < view.offset {
		view.offset = view.cursor
	}
	if view.cursor >= view.offset+lineCount {
		view.offset = view.cursor - lineCount + 1
	}
	lastOffset := max(len(view.drudgers)-lineCount, 0)
	view.offset = min(view.offset, lastOffset)
}

func (view *View) Render() string {
	switch {
	case view.height <= 0:
		return ""
	case !view.hasDrudgers && view.loadErr != nil:
		return view.styles.Error.Render("Could not read Drudgers: " + view.loadErr.Error())
	case !view.hasDrudgers:
		return view.styles.Muted.Render("Loading Drudgers" + cells.Ellipsis)
	case len(view.drudgers) == 0:
		return view.styles.Muted.Render("No Drudgers yet, the first one is built when a task runs")
	}

	lines := []string{view.styles.Header.Render(noCursorMarker + view.layout.Header())}
	for index := view.offset; index < view.rowsEnd(); index++ {
		lines = append(lines, view.renderRow(index))
	}
	return strings.Join(lines, "\n")
}

// measureColumns fits the columns to the cells of every Drudger in the width
// left after the cursor marker.
func (view *View) measureColumns() {
	rows := make([][]string, 0, len(view.drudgers))
	for _, entry := range view.drudgers {
		rows = append(rows, view.newDrudgerRow(entry))
	}
	view.layout = table.NewLayout(columns, rows, view.width-ansi.StringWidth(noCursorMarker))
}

func (view *View) renderRow(index int) string {
	entry := view.drudgers[index]
	row := view.newDrudgerRow(entry)
	cellStyles := view.cellStyles(entry, row)
	if index == view.cursor {
		return view.renderCursorRow(row, cellStyles)
	}
	return noCursorMarker + view.layout.StyledRow(row, cellStyles, lipgloss.NewStyle())
}

// renderCursorRow renders the cells of the cursor row in the styles of a
// normal row, on the selection background. An inner style ends with a reset,
// so every cell, gap and the padding to the view width carries the background
// itself.
func (view *View) renderCursorRow(row []string, cellStyles []lipgloss.Style) string {
	selection := view.styles.Selection
	selectedStyles := make([]lipgloss.Style, 0, len(cellStyles))
	for _, cellStyle := range cellStyles {
		selectedStyles = append(selectedStyles, selection.Inherit(cellStyle))
	}
	line := selection.Render(cursorMarker) + view.layout.StyledRow(row, selectedStyles, selection)
	line = ansi.Truncate(line, view.width, "")
	return line + selection.Render(strings.Repeat(" ", max(view.width-ansi.StringWidth(line), 0)))
}

// cellStyles returns the style of each cell of the row of a Drudger.
func (view *View) cellStyles(entry *drudger.Drudger, row []string) []lipgloss.Style {
	cellStyles := []lipgloss.Style{
		slotCell:        lipgloss.NewStyle(),
		sandboxCell:     view.styles.HealthLabel(row[sandboxCell]),
		agentCell:       view.styles.HealthLabel(row[agentCell]),
		taskCell:        view.styles.ShortID,
		lastCheckedCell: view.styles.Muted,
	}
	if entry.Idle() {
		cellStyles[taskCell] = view.styles.Muted
	}
	return cellStyles
}

// newDrudgerRow returns the cells of the row of a Drudger, in the order of the
// columns.
func (view *View) newDrudgerRow(entry *drudger.Drudger) []string {
	row := make([]string, len(columns))
	row[slotCell] = strconv.Itoa(entry.Slot)
	row[sandboxCell] = sandboxLabel(entry.SandboxHealth)
	row[agentCell] = agentLabel(entry.AgentHealth)
	row[taskCell] = idleLabel
	if !entry.Idle() {
		row[taskCell] = task.ShortID(entry.TaskID)
	}
	row[lastCheckedCell] = formatAgo(entry.LastChecked, view.now)
	return row
}

// sandboxLabel names what DRUDGE last saw of a sandbox. A state this build
// does not know is shown as it is stored.
func sandboxLabel(health drudger.SandboxHealth) string {
	switch health {
	case drudger.SandboxUsable:
		return style.HealthOkLabel
	case drudger.SandboxUnchecked:
		return style.HealthUncheckedLabel
	case drudger.SandboxGone:
		return style.SandboxGoneLabel
	case drudger.SandboxMisplaced:
		return style.SandboxMisplacedLabel
	}
	return string(health)
}

// agentLabel names what DRUDGE last saw of an agent. A state this build does
// not know is shown as it is stored.
func agentLabel(health drudger.AgentHealth) string {
	switch health {
	case drudger.AgentReady:
		return style.HealthOkLabel
	case drudger.AgentUnchecked:
		return style.HealthUncheckedLabel
	case drudger.AgentRefused:
		return style.AgentRefusedLabel
	}
	return string(health)
}

// formatAgo renders roughly how long before now a moment was. A moment after
// now reads as 0s ago.
func formatAgo(moment time.Time, now time.Time) string {
	if moment.IsZero() {
		return neverLabel
	}
	elapsed := max(now.Sub(moment), 0)
	switch {
	case elapsed < time.Minute:
		return fmt.Sprintf(secondsAgoFormat, int(elapsed.Seconds()))
	case elapsed < time.Hour:
		return fmt.Sprintf(minutesAgoFormat, int(elapsed.Minutes()))
	case elapsed < day:
		return fmt.Sprintf(hoursAgoFormat, int(elapsed.Hours()))
	default:
		return fmt.Sprintf(daysAgoFormat, int(elapsed/day))
	}
}

func later(first, second time.Time) time.Time {
	if second.After(first) {
		return second
	}
	return first
}
