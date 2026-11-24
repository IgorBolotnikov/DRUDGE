// Package statuspicker is the box that sets the status of a task.
package statuspicker

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	// Title heads the keys of the picker in the help.
	Title          = "Status"
	cursorMarker   = "› "
	noCursorMarker = "  "
)

// Keys of the picker. The root closes the picker on esc.
var (
	downKey   = key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/k", "move"))
	upKey     = key.NewBinding(key.WithKeys("k", "up"))
	applyKey  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply"))
	cancelKey = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
)

var bindings = []key.Binding{downKey, applyKey, cancelKey}

// Picker lists the statuses of a task in a box centered over the screen. It
// reads the status of the task from the tasks feed and puts the cursor on it
// once the feed has the task.
type Picker struct {
	taskID         task.TaskID
	current        task.TaskStatus
	cursorIndex    int
	isCursorPlaced bool
	helpKey        key.Binding
	boxStyle       lipgloss.Style
	mutedStyle     lipgloss.Style
	screenWidth    int
	screenHeight   int
}

// New returns a picker for the task with id. The cursor sits on the first
// status until the tasks feed has the task. helpKey is the key that opens the
// help, for the hint of the picker.
func New(id task.TaskID, helpKey key.Binding, pickerStyles *styles.Styles) *Picker {
	return &Picker{
		taskID:     id,
		helpKey:    helpKey,
		boxStyle:   pickerStyles.HelpBox,
		mutedStyle: pickerStyles.Muted,
	}
}

func (picker *Picker) SetSize(screenWidth, screenHeight int) {
	picker.screenWidth = screenWidth
	picker.screenHeight = screenHeight
}

// Update moves the cursor on j and k and stops it at the first and the last
// status. Enter closes the picker and sets the status under the cursor. It
// only closes the picker on the current status.
func (picker *Picker) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case views.TasksLoadedMsg:
		picker.handleTasksLoaded(msg)
	case tea.KeyPressMsg:
		return picker.handleKey(msg)
	}
	return nil
}

// handleTasksLoaded keeps the current status of the task. The cursor moves
// onto it the first time the feed has the task and stays where the user put
// it after that.
func (picker *Picker) handleTasksLoaded(msg views.TasksLoadedMsg) {
	index := slices.IndexFunc(msg.Tasks, func(listed task.ListedTask) bool { return listed.Task.ID == picker.taskID })
	if index < 0 {
		return
	}
	picker.current = msg.Tasks[index].Task.Status
	if picker.isCursorPlaced {
		return
	}
	picker.cursorIndex = max(slices.Index(task.Statuses, picker.current), 0)
	picker.isCursorPlaced = true
}

func (picker *Picker) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, downKey):
		picker.cursorIndex = min(picker.cursorIndex+1, len(task.Statuses)-1)
		picker.isCursorPlaced = true
	case key.Matches(msg, upKey):
		picker.cursorIndex = max(picker.cursorIndex-1, 0)
		picker.isCursorPlaced = true
	case key.Matches(msg, applyKey):
		return picker.apply()
	}
	return nil
}

// apply closes the picker with the message that sets the status under the
// cursor. A status drudge maintains itself is forced, and the command asks
// before it starts.
func (picker *Picker) apply() tea.Cmd {
	status := task.Statuses[picker.cursorIndex]
	closeMsg := views.CloseOverlayMsg{}
	if status != picker.current {
		closeMsg.Next = views.SetStatusMsg{TaskID: picker.taskID, Status: status, IsForced: isManaged(status)}
	}
	return func() tea.Msg { return closeMsg }
}

func isManaged(status task.TaskStatus) bool {
	return slices.Contains(task.ManagedStatuses, status)
}

// Render centers the box with the statuses on the screen. The statuses drudge
// maintains itself are muted.
func (picker *Picker) Render() (box string, x, y int) {
	rows := make([]string, 0, len(task.Statuses))
	for index, status := range task.Statuses {
		row := noCursorMarker + string(status)
		if index == picker.cursorIndex {
			row = cursorMarker + string(status)
		}
		if isManaged(status) {
			row = picker.mutedStyle.Render(row)
		}
		rows = append(rows, row)
	}
	box = picker.boxStyle.Render(strings.Join(rows, "\n"))
	x = max((picker.screenWidth-lipgloss.Width(box))/2, 0)
	y = max((picker.screenHeight-lipgloss.Height(box))/2, 0)
	return box, x, y
}

func (picker *Picker) Mouse(msg tea.MouseMsg) tea.Cmd {
	return nil
}

func (picker *Picker) Title() string {
	return Title
}

func (picker *Picker) Keys() []key.Binding {
	return bindings
}

func (picker *Picker) ShortHelp() []key.Binding {
	return []key.Binding{downKey, applyKey, cancelKey, picker.helpKey}
}

func (picker *Picker) TakesAllKeys() bool {
	return false
}

func (picker *Picker) Feeds() []views.Feed {
	return []views.Feed{{Name: views.TasksFeed}}
}
