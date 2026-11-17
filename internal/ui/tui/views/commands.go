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
	RunTaskCommand  CommandName = "run"
	MarkDoneCommand CommandName = "done"
	NewTaskCommand  CommandName = "new"
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

// MarkDoneMsg asks to mark an unmerged task done the way drg task done does.
type MarkDoneMsg struct {
	TaskID task.TaskID
}

func (MarkDoneMsg) CommandName() CommandName { return MarkDoneCommand }

func (msg MarkDoneMsg) Target() string { return task.ShortID(msg.TaskID) }

// newTaskTarget names the target of a new task command. The title of the task
// is known only once the command parsed its draft.
const newTaskTarget = "task"

// NewTaskMsg asks to add the task the draft of a new task describes the way
// drg task new does.
type NewTaskMsg struct{}

func (NewTaskMsg) CommandName() CommandName { return NewTaskCommand }

func (NewTaskMsg) Target() string { return newTaskTarget }

// EditDraftMsg asks the root to open the draft of a new task in the editor.
// The root sends the new task command once the editor exits.
type EditDraftMsg struct{}

// NewTaskKey asks to add a task. It acts on the project, so the Views that
// list the tasks of the project bind it.
var NewTaskKey = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task"))

// EditDraft sends the message that opens the draft of a new task.
func EditDraft() tea.Msg {
	return EditDraftMsg{}
}

var (
	runTaskKey  = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "run"))
	markDoneKey = key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mark done"))
)

// TaskCommandKeys are the keys of the commands on a task. Every View that
// shows tasks binds them and lists them in its keys.
var TaskCommandKeys = []key.Binding{runTaskKey, markDoneKey}

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
	case key.Matches(msg, markDoneKey):
		commandMsg = MarkDoneMsg{TaskID: id}
	default:
		return nil
	}
	return func() tea.Msg { return commandMsg }
}
