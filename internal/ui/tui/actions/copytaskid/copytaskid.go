// Package copytaskid is the command that copies a task ID to the clipboard.
package copytaskid

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const copiedVerdictFormat = "%s copied to clipboard"

// Kind copies the full ID of a task to the clipboard with an OSC52 write to
// the terminal. It needs no service.
var Kind = actions.Kind{Name: views.CopyTaskIDCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	taskID := msg.(views.CopyTaskIDMsg).TaskID
	return actions.DoneMsg{
		Command: command,
		Verdict: fmt.Sprintf(copiedVerdictFormat, command.Target),
		Cmd:     tea.SetClipboard(string(taskID)),
	}
}
