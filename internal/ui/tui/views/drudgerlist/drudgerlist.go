// Package drudgerlist is the View that lists the Drudgers of a project.
package drudgerlist

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	idleLabel  = "idle"
	neverLabel = "never"
)

// Status segments of the Drudger list.
const (
	checkedStatusFormat = "checked %s"
	notCheckedStatus    = "not checked yet"
)

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
	scrollUpKey   = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "scroll"))
	scrollDownKey = key.NewBinding(key.WithKeys("j", "down"))
	jumpTopKey    = key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g/G", "top/bottom"))
	jumpBottomKey = key.NewBinding(key.WithKeys("G", "end"))
)

// Keys are the keys of the Drudger list the help lists.
var Keys = []key.Binding{scrollUpKey, jumpTopKey}

// columns of the Drudger list. The slot always stays. The last check drops
// first, then the task, the agent and the sandbox.
var columns = []table.Column{
	{Title: slotTitle, IsRequired: true},
	{Title: sandboxTitle, Priority: 4},
	{Title: agentTitle, Priority: 3},
	{Title: taskTitle, Priority: 2},
	{Title: lastCheckedTitle, Priority: 1},
}

// View lists the Drudgers of a project as the Drudger file records them. A
// failed read keeps the Drudgers of the last read that worked. Every "ago" of
// the View counts from the latest read or check.
type View struct {
	styles      *styles.Styles
	drudgers    []*drudger.Drudger
	hasDrudgers bool
	loadErr     error
	now         time.Time
	checkedAt   time.Time
	layout      table.Layout
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

// Status says how long ago the Drudgers were last checked, like checked 12s
// ago.
func (view *View) Status() string {
	if view.checkedAt.IsZero() {
		return notCheckedStatus
	}
	return fmt.Sprintf(checkedStatusFormat, formatAgo(view.checkedAt, view.now))
}

func (view *View) SetSize(width, height int) {
	view.width = width
	view.height = height
	view.measureColumns()
	view.scrollTo(view.offset)
}

// Update takes the read Drudgers and the end of a check, and scrolls on keys.
func (view *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case views.DrudgersLoadedMsg:
		view.loadErr = msg.Err
		if msg.Err == nil {
			view.drudgers = msg.Drudgers
			view.hasDrudgers = true
			view.now = later(view.now, msg.ReadAt)
			view.measureColumns()
			view.scrollTo(view.offset)
		}
	case views.DrudgersCheckedMsg:
		if msg.Err == nil {
			view.checkedAt = msg.CheckedAt
			view.now = later(view.now, msg.CheckedAt)
			view.measureColumns()
		}
	case tea.KeyPressMsg:
		view.handleKey(msg)
	}
	return nil
}

func (view *View) handleKey(msg tea.KeyPressMsg) {
	switch {
	case key.Matches(msg, scrollUpKey):
		view.scrollTo(view.offset - 1)
	case key.Matches(msg, scrollDownKey):
		view.scrollTo(view.offset + 1)
	case key.Matches(msg, jumpTopKey):
		view.scrollTo(0)
	case key.Matches(msg, jumpBottomKey):
		view.scrollTo(len(view.drudgers))
	}
}

// rowLineCount returns how many lines the View has for rows, below the
// header.
func (view *View) rowLineCount() int {
	return view.height - 1
}

// scrollTo makes the row at offset the first row in sight. It stops at the
// top and where the last row is on the last line.
func (view *View) scrollTo(offset int) {
	lastOffset := max(len(view.drudgers)-view.rowLineCount(), 0)
	view.offset = min(max(offset, 0), lastOffset)
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

	lines := []string{view.styles.Header.Render(view.layout.Header())}
	end := min(view.offset+view.rowLineCount(), len(view.drudgers))
	for _, entry := range view.drudgers[view.offset:end] {
		lines = append(lines, view.renderRow(entry))
	}
	return strings.Join(lines, "\n")
}

// measureColumns fits the columns to the cells of every Drudger in the width
// of the View.
func (view *View) measureColumns() {
	rows := make([][]string, 0, len(view.drudgers))
	for _, entry := range view.drudgers {
		rows = append(rows, view.newDrudgerRow(entry))
	}
	view.layout = table.NewLayout(columns, rows, view.width)
}

func (view *View) renderRow(entry *drudger.Drudger) string {
	row := view.newDrudgerRow(entry)
	plain := lipgloss.NewStyle()
	cellStyles := []lipgloss.Style{
		slotCell:        plain,
		sandboxCell:     view.styles.HealthLabel(row[sandboxCell]),
		agentCell:       view.styles.HealthLabel(row[agentCell]),
		taskCell:        view.styles.ShortID,
		lastCheckedCell: view.styles.Muted,
	}
	if entry.Idle() {
		cellStyles[taskCell] = view.styles.Muted
	}
	return view.layout.StyledRow(row, cellStyles, plain)
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
