package actions

import tea "charm.land/bubbletea/v2"

// ConfirmMsg asks the root to show a question and send the answer on Answer.
// Answer holds one answer, so the root never blocks on it. The answer is no
// when the TUI quits before the user answers.
type ConfirmMsg struct {
	Question string
	Answer   chan<- bool
}

// Prompt asks the user questions from commands through the program. It needs
// the program attached before any command runs.
type Prompt struct {
	program *tea.Program
}

// Attach sets the program questions go to.
func (prompt *Prompt) Attach(program *tea.Program) {
	prompt.program = program
}

// Confirm sends a question to the program and blocks until the user answers.
func (prompt *Prompt) Confirm(question string) bool {
	answer := make(chan bool, 1)
	prompt.program.Send(ConfirmMsg{Question: question, Answer: answer})
	return <-answer
}
