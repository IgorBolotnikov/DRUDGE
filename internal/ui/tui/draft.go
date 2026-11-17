package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// editorFailedFormat is the error of an editor that exited with an error.
const editorFailedFormat = "editor %s failed: %w"

// editorFunc returns a command that edits the file at path and says when the
// editor exits.
type editorFunc func(path string) tea.Cmd

// draftEditedMsg says the editor of the draft exited, with the error it failed
// with.
type draftEditedMsg struct {
	err error
}

// editorExited returns the callback of a suspended editor that names the
// editor in its error.
func editorExited(editorName string) tea.ExecCallback {
	return func(err error) tea.Msg {
		if err != nil {
			err = fmt.Errorf(editorFailedFormat, editorName, err)
		}
		return draftEditedMsg{err: err}
	}
}

// handleEditDraft opens the draft of a new task in the editor. It refuses
// while a command runs, the same as a command key.
func (model *Model) handleEditDraft() tea.Cmd {
	if model.running != nil {
		model.running.isRefused = true
		return nil
	}
	path, err := model.commands.OpenDraft()
	if err != nil {
		model.showDraftError(err)
		return nil
	}
	return model.runEditor(path)
}

// handleDraftEdited asks for the new task of the draft the editor left.
func (model *Model) handleDraftEdited(msg draftEditedMsg) tea.Cmd {
	if msg.err != nil {
		model.showDraftError(msg.err)
		return nil
	}
	return model.handleCommand(views.NewTaskMsg{})
}

// showDraftError shows an error of the draft or of its editor until the next
// command or esc.
func (model *Model) showDraftError(err error) {
	model.verdictCount++
	model.verdict = &verdict{text: err.Error(), isError: true, number: model.verdictCount}
}
