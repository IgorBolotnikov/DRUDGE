package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// TaskListViewName is the kind of the View that lists tasks.
const TaskListViewName = "task-list"

// Widths of the task list columns.
const (
	statusWidth   = 15
	minTitleWidth = 10
	columnGap     = "  "
	columnCount   = 5
)

// Titles of the task list columns.
const (
	statusTitle    = "STATUS"
	idTitle        = "ID"
	titleTitle     = "TITLE"
	blockedByTitle = "BLOCKED BY"
	ticketTitle    = "TICKET"
)

const (
	// childIndent starts the status and the title of a task listed under its
	// parent.
	childIndent    = "  "
	cursorMarker   = "▸ "
	noCursorMarker = "  "
	blockerJoiner  = ", "
)

// Keys of the task list. The up key and the top key carry the hint of their
// pair.
var (
	moveUpKey     = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "move"))
	moveDownKey   = key.NewBinding(key.WithKeys("j", "down"))
	jumpTopKey    = key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g/G", "top/bottom"))
	jumpBottomKey = key.NewBinding(key.WithKeys("G", "end"))
)

// TaskListView lists the tasks of a project the way drg task list does, with
// a cursor on one row.
type TaskListView struct {
	styles   *Styles
	tasks    []task.ListedTask
	loadErr  error
	isLoaded bool
	cursor   int
	offset   int
	width    int
	height   int
}

// NewTaskListView returns a task list that shows a loading line until it gets
// the tasks.
func NewTaskListView(styles *Styles) *TaskListView {
	return &TaskListView{styles: styles}
}

func (view *TaskListView) Name() string {
	return TaskListViewName
}

func (view *TaskListView) KeyHints() []key.Binding {
	return []key.Binding{moveUpKey, jumpTopKey}
}

func (view *TaskListView) SetSize(width, height int) {
	view.width = width
	view.height = height
	view.scrollToCursor()
}

// Update takes the loaded tasks and moves the cursor on keys.
func (view *TaskListView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tasksLoadedMsg:
		view.isLoaded = true
		view.tasks = msg.tasks
		view.loadErr = msg.err
		view.cursor = 0
		view.offset = 0
	case tea.KeyPressMsg:
		view.moveCursor(msg)
	}
	return nil
}

func (view *TaskListView) moveCursor(msg tea.KeyPressMsg) {
	last := len(view.tasks) - 1
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

// scrollToCursor moves the rows in sight so the cursor row is one of them.
func (view *TaskListView) scrollToCursor() {
	rowCount := view.height - 1
	if rowCount <= 0 {
		return
	}
	if view.cursor < view.offset {
		view.offset = view.cursor
	}
	if view.cursor >= view.offset+rowCount {
		view.offset = view.cursor - rowCount + 1
	}
}

func (view *TaskListView) View() string {
	switch {
	case view.height <= 0:
		return ""
	case !view.isLoaded:
		return view.styles.Muted.Render("Loading tasks" + ellipsis)
	case view.loadErr != nil:
		return view.styles.Error.Render("Could not load tasks: " + view.loadErr.Error())
	case len(view.tasks) == 0:
		return view.styles.Muted.Render("No tasks found")
	}

	columns := view.columns()
	header := taskRow{status: statusTitle, id: idTitle, title: titleTitle, blockedBy: blockedByTitle, ticket: ticketTitle}
	plain := lipgloss.NewStyle()
	lines := []string{view.styles.Header.Render(columns.render(noCursorMarker, header, plain, plain))}
	end := min(view.offset+view.height-1, len(view.tasks))
	for index := view.offset; index < end; index++ {
		lines = append(lines, view.renderRow(columns, index))
	}
	return strings.Join(lines, "\n")
}

// renderRow renders the row at index. The cursor row and a context row take
// one style across the row, so their cells stay plain.
func (view *TaskListView) renderRow(columns taskColumns, index int) string {
	entry := view.tasks[index]
	row := newTaskRow(entry)
	plain := lipgloss.NewStyle()
	switch {
	case index == view.cursor:
		return view.styles.Cursor.Render(fit(columns.render(cursorMarker, row, plain, plain), view.width))
	case entry.IsContext:
		return view.styles.Muted.Render(columns.render(noCursorMarker, row, plain, plain))
	default:
		return columns.render(noCursorMarker, row, view.styles.Status(entry.Task.Status), view.styles.ShortID)
	}
}

// taskRow holds the cells of one row of the task list.
type taskRow struct {
	status    string
	id        string
	title     string
	blockedBy string
	ticket    string
}

func newTaskRow(entry task.ListedTask) taskRow {
	indent := ""
	if entry.IsUnderParent {
		indent = childIndent
	}
	holding := make([]string, 0, len(entry.Holding))
	for _, id := range entry.Holding {
		holding = append(holding, task.ShortID(id))
	}
	return taskRow{
		status:    indent + string(entry.Task.Status),
		id:        task.ShortID(entry.Task.ID),
		title:     indent + entry.Task.Title,
		blockedBy: strings.Join(holding, blockerJoiner),
		ticket:    entry.Task.TicketID,
	}
}

// taskColumns holds the widths of the columns that change with the tasks and
// the size of the View.
type taskColumns struct {
	title     int
	blockedBy int
}

// columns makes the blockers column as wide as its widest value and gives the
// title what is left of the width.
func (view *TaskListView) columns() taskColumns {
	blockedByWidth := ansi.StringWidth(blockedByTitle)
	ticketWidth := ansi.StringWidth(ticketTitle)
	for _, entry := range view.tasks {
		row := newTaskRow(entry)
		blockedByWidth = max(blockedByWidth, ansi.StringWidth(row.blockedBy))
		ticketWidth = max(ticketWidth, ansi.StringWidth(row.ticket))
	}
	gapsWidth := (columnCount - 1) * len(columnGap)
	fixedWidth := len(noCursorMarker) + statusWidth + task.ShortIDLength + blockedByWidth + ticketWidth + gapsWidth
	return taskColumns{
		title:     max(view.width-fixedWidth, minTitleWidth),
		blockedBy: blockedByWidth,
	}
}

func (columns taskColumns) render(marker string, row taskRow, statusStyle, idStyle lipgloss.Style) string {
	return marker +
		statusStyle.Render(fitEllipsis(row.status, statusWidth)) + columnGap +
		idStyle.Render(fitEllipsis(row.id, task.ShortIDLength)) + columnGap +
		fitEllipsis(row.title, columns.title) + columnGap +
		fitEllipsis(row.blockedBy, columns.blockedBy) + columnGap +
		row.ticket
}
