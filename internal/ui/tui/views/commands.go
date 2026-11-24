package views

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// CommandName names a kind of command.
type CommandName string

// Names of the kinds of commands. The bottom bar and the event log show them.
const (
	RunTaskCommand    CommandName = "run"
	RerunTaskCommand  CommandName = "rerun"
	MarkDoneCommand   CommandName = "done"
	NewTaskCommand    CommandName = "new"
	EditTaskCommand   CommandName = "edit"
	RemoveTaskCommand CommandName = "rm"
)

// CommandMsg asks the root to run a command. It names the command and carries
// the IDs it works on. Whether the command is allowed is up to the service
// that runs it.
type CommandMsg interface {
	CommandName() CommandName
	// Target names what the command works on in the bottom bar and the event
	// log, like the short ID of a task.
	Target() string
}

// RunTaskMsg asks to hand a task to a coding agent the way drg task run does.
type RunTaskMsg struct {
	TaskID task.TaskID
}

func (RunTaskMsg) CommandName() CommandName { return RunTaskCommand }

func (msg RunTaskMsg) Target() string { return task.ShortID(msg.TaskID) }

// RerunTaskMsg asks to start a task over the way drg task rerun does.
type RerunTaskMsg struct {
	TaskID task.TaskID
}

func (RerunTaskMsg) CommandName() CommandName { return RerunTaskCommand }

func (msg RerunTaskMsg) Target() string { return task.ShortID(msg.TaskID) }

// MarkDoneMsg asks to mark an unmerged task done the way drg task done does.
type MarkDoneMsg struct {
	TaskID task.TaskID
}

func (MarkDoneMsg) CommandName() CommandName { return MarkDoneCommand }

func (msg MarkDoneMsg) Target() string { return task.ShortID(msg.TaskID) }

// EditTaskMsg asks to open the draft of a task in the editor and change the
// task to what the draft describes the way drg task edit does.
type EditTaskMsg struct {
	TaskID task.TaskID
}

func (EditTaskMsg) CommandName() CommandName { return EditTaskCommand }

func (msg EditTaskMsg) Target() string { return task.ShortID(msg.TaskID) }

// RemoveTaskMsg asks to delete a task the way drg task rm does.
type RemoveTaskMsg struct {
	TaskID task.TaskID
}

func (RemoveTaskMsg) CommandName() CommandName { return RemoveTaskCommand }

func (msg RemoveTaskMsg) Target() string { return task.ShortID(msg.TaskID) }

// newTaskTarget names the target of a new task command. The title of the task
// is known only once the command parsed its draft.
const newTaskTarget = "task"

// NewTaskMsg asks to open the draft of a new task in the editor and add the
// task it describes the way drg task new does.
type NewTaskMsg struct{}

func (NewTaskMsg) CommandName() CommandName { return NewTaskCommand }

func (NewTaskMsg) Target() string { return newTaskTarget }

// NewTaskKey asks to add a task. It acts on the project, so the Views that
// list the tasks of the project bind it.
var NewTaskKey = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task"))

// NewTask sends the message that asks to add a task.
func NewTask() tea.Msg {
	return NewTaskMsg{}
}

var (
	runTaskKey    = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "run"))
	rerunTaskKey  = key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rerun"))
	markDoneKey   = key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mark done"))
	editTaskKey   = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit"))
	removeTaskKey = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "remove"))
)

// TaskCommandKeys are the keys of the commands on a task. Every View that
// shows tasks binds them and lists them in its keys.
var TaskCommandKeys = []key.Binding{runTaskKey, rerunTaskKey, markDoneKey, editTaskKey, removeTaskKey}

// TaskCommand returns the command that sends the command message of a key for
// the task with id. It returns nil for a key of no command and for an empty
// id.
func TaskCommand(msg tea.KeyPressMsg, id task.TaskID) tea.Cmd {
	if id == "" {
		return nil
	}
	var commandMsg CommandMsg
	switch {
	case key.Matches(msg, runTaskKey):
		commandMsg = RunTaskMsg{TaskID: id}
	case key.Matches(msg, rerunTaskKey):
		commandMsg = RerunTaskMsg{TaskID: id}
	case key.Matches(msg, markDoneKey):
		commandMsg = MarkDoneMsg{TaskID: id}
	case key.Matches(msg, editTaskKey):
		commandMsg = EditTaskMsg{TaskID: id}
	case key.Matches(msg, removeTaskKey):
		commandMsg = RemoveTaskMsg{TaskID: id}
	default:
		return nil
	}
	return func() tea.Msg { return commandMsg }
}
