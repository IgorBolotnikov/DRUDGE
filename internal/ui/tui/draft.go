package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/report"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	// draftNameFormat names the draft file of a project in the system temp
	// directory.
	draftNameFormat = "drudge-%s-draft.md"
	// draftFilePerm keeps a draft readable by its owner only, since the temp
	// directory is shared.
	draftFilePerm = 0o600
	editorEnv     = "EDITOR"
	defaultEditor = "vi"
)

// Lines of a draft in the bottom bar.
const (
	nothingChangedText = "nothing changed"
	draftKeptFormat    = "%w · n to reopen your draft"
)

// editorFunc returns a command that edits the file at path and says when the
// editor exits.
type editorFunc func(path string) tea.Cmd

// draftEditedMsg says the editor of the draft exited, with the error it failed
// with.
type draftEditedMsg struct {
	err error
}

// draftFile is the file a new task is written in. A draft that did not turn
// into a task stays in it, and the next draft opens it again.
type draftFile struct {
	path     string
	template string
}

// newDraftFile returns the draft file of a project in the system temp
// directory, with a template that has defaultStatus as its status.
func newDraftFile(projectSlug string, defaultStatus task.TaskStatus) draftFile {
	return draftFile{
		path:     filepath.Join(os.TempDir(), fmt.Sprintf(draftNameFormat, projectSlug)),
		template: task.FormatDraft(task.Draft{Status: defaultStatus}),
	}
}

// open writes the template to the draft file unless the file holds a kept
// draft.
func (draft draftFile) open() error {
	_, err := os.Stat(draft.path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("could not read the draft %s: %w", draft.path, err)
	}
	if err := os.WriteFile(draft.path, []byte(draft.template), draftFilePerm); err != nil {
		return fmt.Errorf("could not write the draft %s: %w", draft.path, err)
	}
	return nil
}

func (draft draftFile) read() (string, error) {
	content, err := os.ReadFile(draft.path)
	if err != nil {
		return "", fmt.Errorf("could not read the draft %s: %w", draft.path, err)
	}
	return string(content), nil
}

func (draft draftFile) remove() error {
	if err := os.Remove(draft.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("could not remove the draft %s: %w", draft.path, err)
	}
	return nil
}

// isUnchanged reports whether content is the template, give or take the blank
// space an editor adds or strips at the ends.
func (draft draftFile) isUnchanged(content string) bool {
	return strings.TrimSpace(content) == strings.TrimSpace(draft.template)
}

// execEditor suspends the program and runs $EDITOR on the file at path, or vi
// when $EDITOR is unset. $EDITOR goes through the shell, so it may carry
// arguments, like code --wait.
func execEditor(path string) tea.Cmd {
	editor := os.Getenv(editorEnv)
	if editor == "" {
		editor = defaultEditor
	}
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			err = fmt.Errorf("editor %s failed: %w", editor, err)
		}
		return draftEditedMsg{err: err}
	})
}

// handleEditDraft opens the draft of a new task in the editor. It refuses
// while a command runs, the same as a command key.
func (model *Model) handleEditDraft() tea.Cmd {
	if model.running != nil {
		model.running.isRefused = true
		return nil
	}
	if err := model.draft.open(); err != nil {
		model.showDraftError(err)
		return nil
	}
	return model.runEditor(model.draft.path)
}

// handleDraftEdited reads the draft the editor left and asks for the new task
// it describes. An empty draft, the template and a draft with no title and no
// description cancel the new task and remove the draft. A draft that does not
// parse stays.
func (model *Model) handleDraftEdited(msg draftEditedMsg) tea.Cmd {
	if msg.err != nil {
		model.showDraftError(msg.err)
		return nil
	}
	content, err := model.draft.read()
	if err != nil {
		model.showDraftError(err)
		return nil
	}
	if strings.TrimSpace(content) == "" || model.draft.isUnchanged(content) {
		return model.cancelDraft()
	}
	draft, err := task.ParseDraft(content)
	if err != nil {
		model.showDraftError(err)
		return nil
	}
	if draft.Title == "" && draft.Description == "" {
		return model.cancelDraft()
	}
	return model.handleCommand(views.NewTaskMsg{Draft: draft})
}

// cancelDraft removes the draft and says nothing changed.
func (model *Model) cancelDraft() tea.Cmd {
	if err := model.draft.remove(); err != nil {
		model.showDraftError(err)
		return nil
	}
	model.verdictCount++
	number := model.verdictCount
	model.verdict = &verdict{text: nothingChangedText, isSkipped: true, number: number}
	return model.startTimer(verdictLifetime, func(time.Time) tea.Msg { return verdictExpiredMsg{number: number} })
}

// showDraftError shows an error that leaves the draft in place until the next
// command or esc.
func (model *Model) showDraftError(err error) {
	model.verdictCount++
	model.verdict = &verdict{text: fmt.Errorf(draftKeptFormat, err).Error(), isError: true, number: model.verdictCount}
}

// closeDraft removes the draft of a new task that worked and points the error
// of one that failed at the kept draft. A draft that cannot be removed is a
// warning of the command, since the task exists.
func (model *Model) closeDraft(msg actions.DoneMsg) actions.DoneMsg {
	if msg.Err != nil {
		msg.Err = fmt.Errorf(draftKeptFormat, msg.Err)
		return msg
	}
	if err := model.draft.remove(); err != nil {
		model.events.add(eventLogEntry{at: time.Now(), command: msg.Command, level: report.LevelWarning, text: err.Error()})
		if model.running != nil {
			model.running.warningCount++
		}
	}
	return msg
}
