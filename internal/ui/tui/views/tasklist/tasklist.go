// Package tasklist is the View that lists the tasks of a project.
package tasklist

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/table"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Name is the kind of the View that lists tasks.
const Name = "task-list"

// Description says in one line what the View shows.
const Description = "Lists the tasks of the project with the rows of drg task list."

// minTitleWidth is the fewest cells the title column takes.
const minTitleWidth = 10

// Titles of the task list columns.
const (
	statusTitle    = "STATUS"
	idTitle        = "ID"
	titleTitle     = "TITLE"
	blockedByTitle = "BLOCKED BY"
	ticketTitle    = "TICKET"
	parentTitle    = "PARENT"
)

// Indexes of the cells of a row, in the order of the columns.
const (
	statusCell = iota
	idCell
	titleCell
	blockedByCell
	ticketCell
	parentCell
)

const (
	// childIndent starts the status and the title of a task listed under its
	// parent.
	childIndent    = "  "
	cursorMarker   = "▸ "
	noCursorMarker = "  "
	blockerJoiner  = ", "
)

// Status segments of the task list: the number of the cursor row and the
// task count.
const (
	statusFormat  = "%d/%d"
	noTasksStatus = "0/0"
)

// Lines above the rows: the filter while it is open and the header.
const (
	filterLines = 1
	headerLines = 1
)

// wheelScrollLines is how many lines one turn of the wheel scrolls.
const wheelScrollLines = 1

// filterPrompt starts the filter input. The input keeps one more cell for its
// cursor.
const (
	filterPrompt    = "/"
	filterCursorGap = 1
)

// StatusOption names the option that shows only the tasks of one status.
const StatusOption = "status"

// Options are the options a Tile sets for the task list.
var Options = []views.Option{
	{
		Name:        StatusOption,
		Type:        views.OptionString,
		Allowed:     statusNames(),
		Description: "Shows only the tasks with this status, without grouping them under their parent. Without it the list shows every task.",
	},
}

// statusNames returns the name of every task status.
func statusNames() []string {
	names := make([]string, 0, len(task.Statuses))
	for _, status := range task.Statuses {
		names = append(names, string(status))
	}
	return names
}

// Keys of the task list. The up key and the top key carry the help of their
// pair. While the filter is open, the arrow keys move the cursor, enter opens
// the task and every other key goes to the filter.
var (
	moveUpKey      = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "move"))
	moveDownKey    = key.NewBinding(key.WithKeys("j", "down"))
	jumpTopKey     = key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g/G", "top/bottom"))
	jumpBottomKey  = key.NewBinding(key.WithKeys("G", "end"))
	openTaskKey    = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open task"))
	openFilterKey  = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"))
	closeFilterKey = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter"))
	arrowUpKey     = key.NewBinding(key.WithKeys("up"))
	arrowDownKey   = key.NewBinding(key.WithKeys("down"))
)

// Keys are the keys of the task list the help lists.
var Keys = []key.Binding{moveUpKey, jumpTopKey, openTaskKey, openFilterKey, closeFilterKey}

// columns of the task list. The status and the title always stay. The parent
// drops first, then the ticket, the blockers and the ID.
var columns = []table.Column{
	{Title: statusTitle, MinWidth: statusWidth(), IsRequired: true},
	{Title: idTitle, MinWidth: task.ShortIDLength, Priority: 4},
	{Title: titleTitle, MinWidth: minTitleWidth, IsFill: true},
	{Title: blockedByTitle, Priority: 3},
	{Title: ticketTitle, Priority: 2},
	{Title: parentTitle, Priority: 1},
}

// statusWidth returns the width of the widest status of a task listed under
// its parent, so the status column keeps its width as the tasks change.
func statusWidth() int {
	width := 0
	for _, status := range task.Statuses {
		width = max(width, ansi.StringWidth(childIndent+string(status)))
	}
	return width
}

// View lists the tasks of a project the way drg task list does, with a cursor
// on one row. A failed load keeps the tasks of the last load that worked. The
// filter shows the tasks whose title holds its text in any case.
type View struct {
	styles      *styles.Styles
	status      task.TaskStatus
	tasks       []task.ListedTask
	rows        []task.ListedTask
	hasTasks    bool
	measured    table.Measured
	layout      table.Layout
	loadErr     error
	filter      textinput.Model
	isFiltering bool
	cursor      int
	offset      int
	width       int
	height      int
}

// New returns a task list that shows a loading line until it gets the tasks.
// A list with a status shows only the tasks with that status. An empty status
// shows every task.
func New(viewStyles *styles.Styles, status task.TaskStatus) *View {
	filter := textinput.New()
	filter.Prompt = filterPrompt
	filter.SetStyles(textinput.Styles{Focused: textinput.StyleState{Prompt: viewStyles.Muted}})
	measured := table.Measure(columns, nil)
	return &View{styles: viewStyles, status: status, filter: filter, measured: measured, layout: measured.Fit(0)}
}

func (view *View) Name() string {
	return Name
}

// TakesAllKeys reports whether the filter is open.
func (view *View) TakesAllKeys() bool {
	return view.isFiltering
}

// Status returns the number of the cursor row and the count of the rows that
// pass the filter, like 12/200. It is empty until the tasks load.
func (view *View) Status() string {
	if !view.hasTasks {
		return ""
	}
	if len(view.rows) == 0 {
		return noTasksStatus
	}
	cursorNumber := view.cursor + 1
	return fmt.Sprintf(statusFormat, cursorNumber, len(view.rows))
}

func (view *View) SetSize(width, height int) {
	view.width = width
	view.height = height
	view.fitColumns()
	view.filter.SetWidth(max(width-ansi.StringWidth(filterPrompt)-filterCursorGap, 0))
	view.scrollToCursor()
}

// Update takes the loaded tasks, moves the cursor and edits the filter on
// keys, moves the cursor to a clicked row and scrolls on the wheel. Moving
// the cursor onto another task selects that task. Enter opens the task under
// the cursor.
func (view *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case views.TasksLoadedMsg:
		view.loadErr = msg.Err
		if msg.Err == nil {
			view.tasks = view.withStatus(msg.Tasks)
			view.hasTasks = true
			view.measureColumns()
			view.showRows()
		}
	case tea.KeyPressMsg:
		if key.Matches(msg, openTaskKey) {
			return view.openTask()
		}
		previousTaskID := view.cursorTaskID()
		var cmd tea.Cmd
		if view.isFiltering {
			cmd = view.handleFilterKey(msg)
		} else {
			cmd = view.handleKey(msg)
		}
		return tea.Batch(cmd, view.selectMovedCursor(previousTaskID))
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

// handleClick moves the cursor to the row at the line of the click. A click
// on the filter, the header or below the last row does nothing.
func (view *View) handleClick(msg tea.MouseClickMsg) {
	rowIndex := view.offset + msg.Y - view.firstRowLine()
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
	lastOffset := max(len(view.rows)-lineCount, 0)
	view.offset = min(max(view.offset+lines, 0), lastOffset)
	lastShownIndex := view.offset + lineCount - 1
	view.cursor = min(max(view.cursor, view.offset), lastShownIndex)
}

// openTask asks to open the task under the cursor. It does nothing when no
// row is shown.
func (view *View) openTask() tea.Cmd {
	taskID := view.cursorTaskID()
	if taskID == "" {
		return nil
	}
	return func() tea.Msg { return views.OpenTaskMsg{TaskID: taskID} }
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

func (view *View) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if key.Matches(msg, openFilterKey) {
		view.isFiltering = true
		view.scrollToCursor()
		return view.filter.Focus()
	}
	view.moveCursor(msg)
	return nil
}

// handleFilterKey closes and clears the filter on esc, moves the cursor on
// the arrow keys and gives every other key to the filter.
func (view *View) handleFilterKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, closeFilterKey):
		view.isFiltering = false
		view.filter.Blur()
		view.filter.Reset()
		view.showRows()
		return nil
	case key.Matches(msg, arrowUpKey, arrowDownKey):
		view.moveCursor(msg)
		return nil
	}
	var cmd tea.Cmd
	view.filter, cmd = view.filter.Update(msg)
	view.showRows()
	return cmd
}

// withStatus returns the tasks with the status of the list, each at the top
// level. A list with no status returns every task as it is.
func (view *View) withStatus(tasks []task.ListedTask) []task.ListedTask {
	if view.status == "" {
		return tasks
	}
	var kept []task.ListedTask
	for _, entry := range tasks {
		if entry.Task.Status == view.status {
			kept = append(kept, task.ListedTask{Task: entry.Task, Holding: entry.Holding})
		}
	}
	return kept
}

// showRows shows the tasks the filter matches and keeps the cursor on the
// task it was on. When that task is not shown, the cursor keeps its row or
// moves up to the last row.
func (view *View) showRows() {
	cursorTaskID := view.cursorTaskID()
	needle := strings.ToLower(view.filter.Value())
	view.rows = slices.DeleteFunc(slices.Clone(view.tasks), func(entry task.ListedTask) bool {
		return !strings.Contains(strings.ToLower(entry.Task.Title), needle)
	})
	index := slices.IndexFunc(view.rows, func(entry task.ListedTask) bool { return entry.Task.ID == cursorTaskID })
	if index == -1 {
		index = max(min(view.cursor, len(view.rows)-1), 0)
	}
	view.cursor = index
	view.scrollToCursor()
}

// cursorTaskID returns the ID of the task under the cursor, or an empty ID
// when no row is shown.
func (view *View) cursorTaskID() task.TaskID {
	if view.cursor >= len(view.rows) {
		return ""
	}
	return view.rows[view.cursor].Task.ID
}

func (view *View) moveCursor(msg tea.KeyPressMsg) {
	last := len(view.rows) - 1
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

// rowLineCount returns how many lines the View has for rows, below the filter
// and the header.
func (view *View) rowLineCount() int {
	return view.height - view.firstRowLine()
}

// firstRowLine returns the index of the line of the first row in sight, below
// the filter and the header.
func (view *View) firstRowLine() int {
	if view.isFiltering {
		return filterLines + headerLines
	}
	return headerLines
}

// rowsEnd returns the index after the last row in sight.
func (view *View) rowsEnd() int {
	return min(view.offset+view.rowLineCount(), len(view.rows))
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
	lastOffset := max(len(view.rows)-lineCount, 0)
	view.offset = min(view.offset, lastOffset)
}

func (view *View) Render() string {
	switch {
	case view.height <= 0:
		return ""
	case view.isFiltering:
		return view.filter.View() + "\n" + view.renderList()
	}
	return view.renderList()
}

// renderList renders the header and the rows in sight, or a line that says
// why there are none.
func (view *View) renderList() string {
	switch {
	case !view.hasTasks && view.loadErr != nil:
		return view.styles.Error.Render("Could not load tasks: " + view.loadErr.Error())
	case !view.hasTasks:
		return view.styles.Muted.Render("Loading tasks" + cells.Ellipsis)
	case len(view.tasks) == 0:
		return view.styles.Muted.Render("No tasks found")
	case len(view.rows) == 0:
		return view.styles.Muted.Render("No tasks match the filter")
	}

	lines := []string{view.styles.Header.Render(noCursorMarker + view.layout.Header())}
	for index := view.offset; index < view.rowsEnd(); index++ {
		lines = append(lines, view.renderRow(index))
	}
	return strings.Join(lines, "\n")
}

// measureColumns finds the width of each column from the cells of every task,
// so the filter does not move the columns.
func (view *View) measureColumns() {
	rows := make([][]string, 0, len(view.tasks))
	for _, entry := range view.tasks {
		rows = append(rows, newTaskRow(entry))
	}
	view.measured = table.Measure(columns, rows)
	view.fitColumns()
}

// fitColumns fits the measured columns in the width left after the cursor
// marker.
func (view *View) fitColumns() {
	view.layout = view.measured.Fit(view.width - ansi.StringWidth(noCursorMarker))
}

// renderRow renders the row at index. A context row takes one style across
// the row, so its cells stay plain.
func (view *View) renderRow(index int) string {
	entry := view.rows[index]
	row := newTaskRow(entry)
	switch {
	case index == view.cursor:
		return view.renderCursorRow(entry, row)
	case entry.IsContext:
		return view.styles.Muted.Render(noCursorMarker + view.layout.Row(row))
	}
	row[statusCell] = view.styles.Status(entry.Task.Status).Render(row[statusCell])
	row[idCell] = view.styles.ShortID.Render(row[idCell])
	return noCursorMarker + view.layout.Row(row)
}

// renderCursorRow renders the cells of the cursor row in the styles of a
// normal row, on the selection background. An inner style ends with a reset,
// so every cell, gap and the padding to the view width carries the background
// itself.
func (view *View) renderCursorRow(entry task.ListedTask, row []string) string {
	selection := view.styles.Selection
	cellStyles := make([]lipgloss.Style, len(row))
	for index := range cellStyles {
		cellStyles[index] = selection
		if entry.IsContext {
			cellStyles[index] = selection.Inherit(view.styles.Muted)
		}
	}
	if !entry.IsContext {
		cellStyles[statusCell] = selection.Inherit(view.styles.Status(entry.Task.Status))
		cellStyles[idCell] = selection.Inherit(view.styles.ShortID)
	}
	line := selection.Render(cursorMarker) + view.layout.StyledRow(row, cellStyles, selection)
	line = ansi.Truncate(line, view.width, "")
	return line + selection.Render(strings.Repeat(" ", max(view.width-ansi.StringWidth(line), 0)))
}

// newTaskRow returns the cells of the row of a task, in the order of the
// columns.
func newTaskRow(entry task.ListedTask) []string {
	indent := ""
	if entry.IsUnderParent {
		indent = childIndent
	}
	holding := make([]string, 0, len(entry.Holding))
	for _, id := range entry.Holding {
		holding = append(holding, task.ShortID(id))
	}
	row := make([]string, len(columns))
	row[statusCell] = indent + string(entry.Task.Status)
	row[idCell] = task.ShortID(entry.Task.ID)
	row[titleCell] = indent + entry.Task.Title
	row[blockedByCell] = strings.Join(holding, blockerJoiner)
	row[ticketCell] = entry.Task.TicketID
	if entry.Task.ParentTaskID != "" {
		row[parentCell] = task.ShortID(entry.Task.ParentTaskID)
	}
	return row
}
