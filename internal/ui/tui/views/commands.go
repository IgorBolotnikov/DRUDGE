package views

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// CommandName names a kind of command.
type CommandName string

// Names of the kinds of commands. The bottom bar and the event log show them.
const (
	RunTaskCommand      CommandName = "run"
	RerunTaskCommand    CommandName = "rerun"
	MarkDoneCommand     CommandName = "done"
	NewTaskCommand      CommandName = "new"
	EditTaskCommand     CommandName = "edit"
	RemoveTaskCommand   CommandName = "rm"
	CopyTaskIDCommand   CommandName = "copy"
	SetStatusCommand    CommandName = "status"
	PullRequestsCommand CommandName = "pr"
	ReclaimCommand      CommandName = "reclaim"
	NukeCommand         CommandName = "nuke"
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

// CopyTaskIDMsg asks to copy the ID of a task to the clipboard.
type CopyTaskIDMsg struct {
	TaskID task.TaskID
}

func (CopyTaskIDMsg) CommandName() CommandName { return CopyTaskIDCommand }

func (msg CopyTaskIDMsg) Target() string { return task.ShortID(msg.TaskID) }

// SetStatusMsg asks to set the status of a task the way drg task edit --status
// does. IsForced sets a status drudge maintains itself, the way --force does.
type SetStatusMsg struct {
	TaskID   task.TaskID
	Status   task.TaskStatus
	IsForced bool
}

func (SetStatusMsg) CommandName() CommandName { return SetStatusCommand }

func (msg SetStatusMsg) Target() string { return task.ShortID(msg.TaskID) }

// PickStatusMsg asks the root to open the status picker for a task.
type PickStatusMsg struct {
	TaskID task.TaskID
}

// PullRequestsMsg asks to open the pull requests close-out left behind for a
// task the way drg task pr does.
type PullRequestsMsg struct {
	TaskID task.TaskID
}

func (PullRequestsMsg) CommandName() CommandName { return PullRequestsCommand }

func (msg PullRequestsMsg) Target() string { return task.ShortID(msg.TaskID) }

// newTaskTarget names the target of a new task command. The title of the task
// is known only once the command parsed its draft.
const newTaskTarget = "task"

// NewTaskMsg asks to open the draft of a new task in the editor and add the
// task it describes the way drg task new does.
type NewTaskMsg struct{}

func (NewTaskMsg) CommandName() CommandName { return NewTaskCommand }

func (NewTaskMsg) Target() string { return newTaskTarget }

// reclaimTarget names the target of a reclaim command. It acts on every slot
// of the project.
const reclaimTarget = "slots"

// ReclaimMsg asks to free the Drudger slots whose agent is gone the way drg
// drudger reclaim does.
type ReclaimMsg struct{}

func (ReclaimMsg) CommandName() CommandName { return ReclaimCommand }

func (ReclaimMsg) Target() string { return reclaimTarget }

const nukeTargetFormat = "Drudger %d"

// NukeMsg asks to delete the Drudger in a slot the way drg drudger nuke does.
// TaskID is the task the View last saw the Drudger work on, and is empty for
// an idle Drudger. IsForced kills the agent of a busy Drudger.
type NukeMsg struct {
	Slot     int
	TaskID   task.TaskID
	IsForced bool
}

func (NukeMsg) CommandName() CommandName { return NukeCommand }

func (msg NukeMsg) Target() string { return fmt.Sprintf(nukeTargetFormat, msg.Slot) }

// NewTaskKey asks to add a task. It acts on the project, so the Views that
// list the tasks of the project bind it.
var NewTaskKey = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task"))

// NewTask sends the message that asks to add a task.
func NewTask() tea.Msg {
	return NewTaskMsg{}
}

var (
	runTaskKey      = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "run"))
	rerunTaskKey    = key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rerun"))
	markDoneKey     = key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mark done"))
	editTaskKey     = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit"))
	removeTaskKey   = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "remove"))
	copyTaskIDKey   = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy id"))
	pickStatusKey   = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "set status"))
	pullRequestsKey = key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "open prs"))
)

// TaskCommandKeys are the keys of the commands on a task. Every View that
// shows tasks binds them and lists them in its keys.
var TaskCommandKeys = []key.Binding{runTaskKey, rerunTaskKey, markDoneKey, editTaskKey, removeTaskKey, copyTaskIDKey, pickStatusKey, pullRequestsKey}

// TaskCommand returns the command that sends the message of a key for the task
// with id. It is a command message, or the message that opens the status
// picker. It returns nil for a key of no command and for an empty
// id.
func TaskCommand(msg tea.KeyPressMsg, id task.TaskID) tea.Cmd {
	if id == "" {
		return nil
	}
	var commandMsg tea.Msg
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
	case key.Matches(msg, copyTaskIDKey):
		commandMsg = CopyTaskIDMsg{TaskID: id}
	case key.Matches(msg, pickStatusKey):
		commandMsg = PickStatusMsg{TaskID: id}
	case key.Matches(msg, pullRequestsKey):
		commandMsg = PullRequestsMsg{TaskID: id}
	default:
		return nil
	}
	return func() tea.Msg { return commandMsg }
}
