package exec

import (
	"os"
	osexec "os/exec"
)

const (
	editorEnv     = "EDITOR"
	defaultEditor = "vi"
)

// Editor builds the processes that edit a file in the editor of the user.
type Editor struct {
	name string
}

// NewEditor returns the editor $EDITOR names, or vi when $EDITOR is unset.
func NewEditor() *Editor {
	name := os.Getenv(editorEnv)
	if name == "" {
		name = defaultEditor
	}
	return &Editor{name: name}
}

// Name returns the editor as $EDITOR gives it, arguments included.
func (editor *Editor) Name() string {
	return editor.name
}

// Command returns the process that edits the file at path. The editor runs
// through the shell, so $EDITOR may carry arguments, like code --wait. The
// path is a positional argument of the shell, so it is never parsed as shell
// code.
func (editor *Editor) Command(path string) *osexec.Cmd {
	return osexec.Command("sh", "-c", editor.name+` "$1"`, "sh", path)
}
