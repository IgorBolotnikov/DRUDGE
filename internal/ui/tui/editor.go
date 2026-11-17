package tui

import (
	osexec "os/exec"

	tea "charm.land/bubbletea/v2"
)

// processEditor builds the process that edits a file.
type processEditor interface {
	Name() string
	Command(path string) *osexec.Cmd
}

// terminalEditor runs an editor in the terminal of the program.
type terminalEditor struct {
	editor processEditor
}

func (terminal terminalEditor) Name() string {
	return terminal.editor.Name()
}

func (terminal terminalEditor) Edit(path string, done tea.ExecCallback) tea.Cmd {
	return tea.ExecProcess(terminal.editor.Command(path), done)
}
