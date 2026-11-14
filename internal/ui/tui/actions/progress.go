package actions

import tea "charm.land/bubbletea/v2"

// ProgressMsg carries one event a service reported while a command ran.
type ProgressMsg struct {
	Event any
}

// Progress sends every event a service reports into the program as a
// message. It needs the program attached before any command runs.
type Progress struct {
	program *tea.Program
}

// Attach sets the program events go to.
func (progress *Progress) Attach(program *tea.Program) {
	progress.program = program
}

// Report sends an event to the program. It blocks until the program takes the
// message, so a service must report from outside the UI loop.
func (progress *Progress) Report(event any) {
	progress.program.Send(ProgressMsg{Event: event})
}
