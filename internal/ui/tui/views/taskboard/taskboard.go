// Package taskboard is the View that shows the tasks of a project as a board
// with one column per task status.
package taskboard

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Name is the kind of the View that shows the task board.
const Name = "task-board"

// Description says in one line what the View shows.
const Description = "Shows the tasks of the project on a board with one column per status."

// minTitleWidth is the fewest cells a column keeps for the titles of its
// tasks. A board too narrow to keep them in every column turns compact.
const minTitleWidth = 15

const (
	cursorMarker   = "▸ "
	noCursorMarker = "  "
	columnGap      = " "
	// headerFormat shows a status and the count of its tasks, like todo 3.
	headerFormat = "%s %d"
)

// statusFormat is the status segment of the board: the status of the cursor column, the number of
// the cursor row and the task count of the column.
const statusFormat = "%s %d/%d"

const headerLines = 1

// headerRow marks a line of the compact board that shows the header of a
// column.
const headerRow = -1

// Keys of the board. The up key, the left key and the top key carry the help of
// their pair.
var (
	moveUpKey     = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "move"))
	moveDownKey   = key.NewBinding(key.WithKeys("j", "down"))
	moveLeftKey   = key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/l", "column"))
	moveRightKey  = key.NewBinding(key.WithKeys("l", "right"))
	jumpTopKey    = key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g/G", "top/bottom"))
	jumpBottomKey = key.NewBinding(key.WithKeys("G", "end"))
	openTaskKey   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open task"))
	openRunLogKey = key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "runlog"))
)

// Keys are the keys of the board the help lists.
var Keys = []key.Binding{moveLeftKey, moveUpKey, jumpTopKey, openTaskKey, openRunLogKey}

// column holds the tasks of one status in the order of the listing.
type column struct {
	status task.TaskStatus
	tasks  []*task.Task
}

// boardLine is what one line of the compact board shows: the header of a
// column or a task of the cursor column.
type boardLine struct {
	columnIndex int
	rowIndex    int
}

// View shows the tasks of a project in one column per known status, with a
// cursor on one task of one column. A task with an unknown status shows in no
// column. A failed load keeps the tasks of the last load that worked. When the
// columns get too narrow for titles, every status shows as a header with a
// count and only the cursor column lists its tasks.
type View struct {
	styles      *styles.Styles
	columns     []column
	hasTasks    bool
	loadErr     error
	columnIndex int
	rowIndex    int
	offset      int
	width       int
	height      int
}

// New returns a board that shows a loading line until it gets the tasks.
func New(viewStyles *styles.Styles) *View {
	return &View{styles: viewStyles, columns: groupByStatus(nil)}
}

func (view *View) Name() string {
	return Name
}

// TakesAllKeys reports false. The board has no input that takes keys.
func (view *View) TakesAllKeys() bool {
	return false
}

// Status returns the status of the cursor column, the number of the cursor row
// and the task count of the column, like todo 2/5. It is empty until the tasks
// load.
func (view *View) Status() string {
	if !view.hasTasks {
		return ""
	}
	cursorColumn := view.columns[view.columnIndex]
	rowNumber := 0
	if len(cursorColumn.tasks) > 0 {
		rowNumber = view.rowIndex + 1
	}
	return fmt.Sprintf(statusFormat, cursorColumn.status, rowNumber, len(cursorColumn.tasks))
}

// Feeds returns the tasks feed and the Sessions feed. The View ignores the
// messages of the Sessions feed, which records the outcome of each finished
// Session on its task for the tasks feed to show.
func (view *View) Feeds() []views.Feed {
	return []views.Feed{{Name: views.TasksFeed}, {Name: views.SessionsFeed}}
}

func (view *View) SetSize(width, height int) {
	view.width = width
	view.height = height
	view.scrollToCursor()
}

// Update takes the loaded tasks, moves the cursor on keys, to a clicked task
// or column and along the column on the wheel. Moving the cursor onto another
// task selects that task. Enter opens the task under the cursor and L opens
// its runlog.
func (view *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case views.TasksLoadedMsg:
		view.loadErr = msg.Err
		if msg.Err == nil {
			view.showTasks(msg.Tasks)
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, openTaskKey):
			return view.openTask()
		case key.Matches(msg, openRunLogKey):
			return view.openRunLog()
		}
		previousTaskID := view.cursorTaskID()
		view.handleKey(msg)
		return view.selectMovedCursor(previousTaskID)
	case tea.MouseClickMsg:
		previousTaskID := view.cursorTaskID()
		view.handleClick(msg)
		return view.selectMovedCursor(previousTaskID)
	case tea.MouseWheelMsg:
		previousTaskID := view.cursorTaskID()
		view.handleWheel(msg)
		return view.selectMovedCursor(previousTaskID)
	}
	return nil
}

// showTasks groups the tasks into the columns and keeps the cursor on the task
// it was on, in whatever column it now is. When that task is gone, the cursor
// keeps its column and its row or moves up to the last row.
func (view *View) showTasks(tasks []task.ListedTask) {
	cursorTaskID := view.cursorTaskID()
	view.columns = groupByStatus(tasks)
	view.hasTasks = true
	for columnIndex, boardColumn := range view.columns {
		rowIndex := slices.IndexFunc(boardColumn.tasks, func(listed *task.Task) bool { return listed.ID == cursorTaskID })
		if rowIndex != -1 {
			view.columnIndex = columnIndex
			view.rowIndex = rowIndex
			view.scrollToCursor()
			return
		}
	}
	view.moveToRow(view.rowIndex)
}

// groupByStatus returns a column for each known status in the order of the
// statuses, with the tasks of that status.
func groupByStatus(tasks []task.ListedTask) []column {
	columns := make([]column, 0, len(task.Statuses))
	for _, status := range task.Statuses {
		boardColumn := column{status: status}
		for _, entry := range tasks {
			if entry.Task.Status == status && !entry.IsContext {
				boardColumn.tasks = append(boardColumn.tasks, entry.Task)
			}
		}
		columns = append(columns, boardColumn)
	}
	return columns
}

func (view *View) handleKey(msg tea.KeyPressMsg) {
	switch {
	case key.Matches(msg, moveLeftKey):
		view.moveToColumn(view.columnIndex - 1)
	case key.Matches(msg, moveRightKey):
		view.moveToColumn(view.columnIndex + 1)
	case key.Matches(msg, moveUpKey):
		view.moveToRow(view.rowIndex - 1)
	case key.Matches(msg, moveDownKey):
		view.moveToRow(view.rowIndex + 1)
	case key.Matches(msg, jumpTopKey):
		view.moveToRow(0)
	case key.Matches(msg, jumpBottomKey):
		view.moveToRow(len(view.columns[view.columnIndex].tasks) - 1)
	}
}

// handleClick moves the cursor to the clicked task, or to the clicked column
// when the click is on a header. A click on a gap or below the last task does
// nothing.
func (view *View) handleClick(msg tea.MouseClickMsg) {
	if !view.hasTasks {
		return
	}
	if view.isCompact() {
		lines := view.compactLines()
		if msg.Y >= len(lines) {
			return
		}
		view.moveTo(lines[msg.Y])
		return
	}
	columnIndex, ok := view.columnAt(msg.X)
	if !ok {
		return
	}
	if msg.Y < headerLines {
		view.moveToColumn(columnIndex)
		return
	}
	rowIndex := view.columnOffset(columnIndex) + msg.Y - headerLines
	if rowIndex < len(view.columns[columnIndex].tasks) {
		view.moveTo(boardLine{columnIndex: columnIndex, rowIndex: rowIndex})
	}
}

// handleWheel moves the cursor one row along its column on a turn of the
// wheel.
func (view *View) handleWheel(msg tea.MouseWheelMsg) {
	switch msg.Button {
	case tea.MouseWheelUp:
		view.moveToRow(view.rowIndex - 1)
	case tea.MouseWheelDown:
		view.moveToRow(view.rowIndex + 1)
	}
}

// moveTo moves the cursor to a column, and to a row of it unless the line is a
// header.
func (view *View) moveTo(line boardLine) {
	view.moveToColumn(line.columnIndex)
	if line.rowIndex != headerRow {
		view.moveToRow(line.rowIndex)
	}
}

// moveToColumn moves the cursor to the column at columnIndex, stopping at the
// first and the last column. The cursor keeps its row or moves up to the last
// row of the column.
func (view *View) moveToColumn(columnIndex int) {
	lastColumnIndex := len(view.columns) - 1
	view.columnIndex = min(max(columnIndex, 0), lastColumnIndex)
	view.offset = 0
	view.moveToRow(view.rowIndex)
}

// moveToRow moves the cursor to rowIndex in its column, stopping at the first
// and the last row.
func (view *View) moveToRow(rowIndex int) {
	lastRowIndex := len(view.columns[view.columnIndex].tasks) - 1
	view.rowIndex = max(min(rowIndex, lastRowIndex), 0)
	view.scrollToCursor()
}

// openTask asks to open the task under the cursor. It does nothing in an empty
// column.
func (view *View) openTask() tea.Cmd {
	taskID := view.cursorTaskID()
	if taskID == "" {
		return nil
	}
	return func() tea.Msg { return views.OpenTaskMsg{TaskID: taskID} }
}

// openRunLog asks to open the runlog of the task under the cursor. It does
// nothing in an empty column.
func (view *View) openRunLog() tea.Cmd {
	taskID := view.cursorTaskID()
	if taskID == "" {
		return nil
	}
	return func() tea.Msg { return views.OpenRunLogMsg{TaskID: taskID} }
}

// selectMovedCursor selects the task under the cursor when it is another task
// than the one at previousTaskID.
func (view *View) selectMovedCursor(previousTaskID task.TaskID) tea.Cmd {
	taskID := view.cursorTaskID()
	if taskID == "" || taskID == previousTaskID {
		return nil
	}
	return func() tea.Msg { return views.TaskSelectedMsg{TaskID: taskID} }
}

// cursorTaskID returns the ID of the task under the cursor, or an empty ID in
// an empty column.
func (view *View) cursorTaskID() task.TaskID {
	tasks := view.columns[view.columnIndex].tasks
	if view.rowIndex >= len(tasks) {
		return ""
	}
	return tasks[view.rowIndex].ID
}

// rowLineCount returns how many lines the cursor column has for tasks: below
// the headers of every status on a compact board, below its header otherwise.
func (view *View) rowLineCount() int {
	if view.isCompact() {
		return view.height - len(view.columns)
	}
	return view.height - headerLines
}

// scrollToCursor moves the tasks in sight of the cursor column so the cursor
// task is one of them. It scrolls back when tasks fit below the last task.
func (view *View) scrollToCursor() {
	lineCount := view.rowLineCount()
	if lineCount <= 0 {
		return
	}
	if view.rowIndex < view.offset {
		view.offset = view.rowIndex
	}
	if view.rowIndex >= view.offset+lineCount {
		view.offset = view.rowIndex - lineCount + 1
	}
	lastOffset := max(len(view.columns[view.columnIndex].tasks)-lineCount, 0)
	view.offset = min(view.offset, lastOffset)
}

// columnOffset returns the index of the first task in sight of a column. Only
// the cursor column scrolls.
func (view *View) columnOffset(columnIndex int) int {
	if columnIndex == view.columnIndex {
		return view.offset
	}
	return 0
}

// columnWidths returns the width of each column. The cells the columns cannot
// share evenly go to the first columns.
func (view *View) columnWidths() []int {
	gapsWidth := ansi.StringWidth(columnGap) * (len(view.columns) - 1)
	sharedWidth := max(view.width-gapsWidth, 0)
	baseWidth := sharedWidth / len(view.columns)
	extraCells := sharedWidth - baseWidth*len(view.columns)
	widths := make([]int, len(view.columns))
	for index := range widths {
		widths[index] = baseWidth
		if index < extraCells {
			widths[index]++
		}
	}
	return widths
}

// isCompact reports whether the narrowest column has less room for titles than
// the board keeps.
func (view *View) isCompact() bool {
	narrowestWidth := slices.Min(view.columnWidths())
	return narrowestWidth-ansi.StringWidth(noCursorMarker) < minTitleWidth
}

// columnAt returns the index of the column at cell x. ok is false for a gap
// between columns.
func (view *View) columnAt(x int) (columnIndex int, ok bool) {
	columnStart := 0
	for index, width := range view.columnWidths() {
		if x >= columnStart && x < columnStart+width {
			return index, true
		}
		columnStart += width + ansi.StringWidth(columnGap)
	}
	return 0, false
}

// compactLines returns what each line of the compact board shows: the header
// of every status, with the tasks in sight of the cursor column below its
// header.
func (view *View) compactLines() []boardLine {
	var lines []boardLine
	for columnIndex, boardColumn := range view.columns {
		lines = append(lines, boardLine{columnIndex: columnIndex, rowIndex: headerRow})
		if columnIndex != view.columnIndex {
			continue
		}
		rowsEnd := min(view.offset+max(view.rowLineCount(), 0), len(boardColumn.tasks))
		for rowIndex := view.offset; rowIndex < rowsEnd; rowIndex++ {
			lines = append(lines, boardLine{columnIndex: columnIndex, rowIndex: rowIndex})
		}
	}
	return lines
}

func (view *View) Render() string {
	switch {
	case view.height <= 0:
		return ""
	case !view.hasTasks && view.loadErr != nil:
		return view.styles.Error.Render("Could not load tasks: " + view.loadErr.Error())
	case !view.hasTasks:
		return view.styles.Muted.Render("Loading tasks" + cells.Ellipsis)
	case view.isCompact():
		return view.renderCompact()
	}
	return view.renderColumns()
}

// renderColumns renders the header and the tasks in sight of every column side
// by side.
func (view *View) renderColumns() string {
	widths := view.columnWidths()
	lines := make([]string, 0, view.height)
	for lineIndex := range view.height {
		lineCells := make([]string, 0, len(view.columns))
		for columnIndex := range view.columns {
			width := widths[columnIndex]
			if lineIndex < headerLines {
				lineCells = append(lineCells, view.renderHeader(columnIndex, width))
				continue
			}
			rowIndex := view.columnOffset(columnIndex) + lineIndex - headerLines
			lineCells = append(lineCells, view.renderTask(boardLine{columnIndex: columnIndex, rowIndex: rowIndex}, width))
		}
		lines = append(lines, strings.Join(lineCells, columnGap))
	}
	return strings.Join(lines, "\n")
}

// renderCompact renders the header of every status, with the tasks in sight of
// the cursor column below its header.
func (view *View) renderCompact() string {
	lines := make([]string, 0, view.height)
	for _, line := range view.compactLines() {
		if line.rowIndex == headerRow {
			lines = append(lines, view.renderHeader(line.columnIndex, view.width))
			continue
		}
		lines = append(lines, view.renderTask(line, view.width))
	}
	return strings.Join(lines[:min(len(lines), view.height)], "\n")
}

// renderHeader renders the status of a column and the count of its tasks
// centered, in the style of the status. The header of the cursor column is reversed, so an
// empty cursor column still shows the cursor.
func (view *View) renderHeader(columnIndex int, width int) string {
	boardColumn := view.columns[columnIndex]
	header := fmt.Sprintf(headerFormat, boardColumn.status, len(boardColumn.tasks))
	headerStyle := view.styles.Status(boardColumn.status).Bold(true)
	if columnIndex == view.columnIndex {
		headerStyle = headerStyle.Reverse(true)
	}
	return headerStyle.Render(cells.CenterEllipsis(header, width))
}

// renderTask renders the title of the task at a line, cut to width. The cursor
// task carries the marker on the selection background. A line below the last
// task of its column is blank.
func (view *View) renderTask(line boardLine, width int) string {
	tasks := view.columns[line.columnIndex].tasks
	if line.rowIndex >= len(tasks) {
		return strings.Repeat(" ", width)
	}
	titleWidth := max(width-ansi.StringWidth(noCursorMarker), 0)
	title := cells.FitEllipsis(tasks[line.rowIndex].Title, titleWidth)
	isCursor := line.columnIndex == view.columnIndex && line.rowIndex == view.rowIndex
	if isCursor {
		return view.styles.Selection.Render(cells.Fit(cursorMarker+title, width))
	}
	return cells.Fit(noCursorMarker+title, width)
}
