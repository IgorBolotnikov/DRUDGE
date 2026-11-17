package tui

import (
	"io"

	tea "charm.land/bubbletea/v2"
)

// fileEditor runs the editor of the user on a file and waits for it to exit.
type fileEditor interface {
	Name() string
	Edit(path string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
}

// terminalEditor runs an editor in the terminal of the program.
type terminalEditor struct {
	editor fileEditor
}

func (terminal terminalEditor) Name() string {
	return terminal.editor.Name()
}

func (terminal terminalEditor) Edit(path string, done tea.ExecCallback) tea.Cmd {
	return tea.Exec(&editorProcess{editor: terminal.editor, path: path}, done)
}

// editorProcess hands the terminal of the program to one run of the editor.
type editorProcess struct {
	editor fileEditor
	path   string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (process *editorProcess) Run() error {
	return process.editor.Edit(process.path, process.stdin, process.stdout, process.stderr)
}

func (process *editorProcess) SetStdin(stdin io.Reader) {
	process.stdin = stdin
}

func (process *editorProcess) SetStdout(stdout io.Writer) {
	process.stdout = stdout
}

func (process *editorProcess) SetStderr(stderr io.Writer) {
	process.stderr = stderr
}
