package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/report"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// verdictLifetime is how long the bottom bar shows the verdict of a command
// that worked.
const verdictLifetime = 5 * time.Second

// Lines of a command in the bottom bar.
const (
	firstStepText        = "starting"
	runningFormat        = "%s %s: %s…"
	refusalFormat        = "wait, still running %s"
	successVerdictPrefix = "✓ "
	skippedVerdictPrefix = "· "
	errorVerdictPrefix   = "✗ "
	warningCountFormat   = " · %s, ! to see"
	hintSeparator        = " · "
)

// timerFunc returns a command that sends the message of fn once d is up.
type timerFunc func(d time.Duration, fn func(time.Time) tea.Msg) tea.Cmd

// runningCommand is what the root keeps of the command that runs.
type runningCommand struct {
	command actions.Command
	// step is the text of the latest step the command reported.
	step string
	// isRefused is whether a second command was refused since the latest
	// step.
	isRefused    bool
	warningCount int
}

// verdict is the outcome of the command that ran last.
type verdict struct {
	text    string
	isError bool
	// isSkipped is whether the command did nothing, like a new task whose
	// draft is unchanged.
	isSkipped    bool
	warningCount int
	// number tells the timer of this verdict from the timers of earlier ones.
	number int
}

// verdictExpiredMsg says the verdict with number has been shown for its
// lifetime.
type verdictExpiredMsg struct {
	number int
}

// nameTask names a task in the text of a command line by its short ID.
func nameTask(named *task.Task) string {
	return task.ShortID(named.ID)
}

// commandConfirmedMsg says the user answered yes to the question of a command.
type commandConfirmedMsg struct {
	msg views.CommandMsg
}

// handleCommand starts the command a View asks for. A command with a question
// opens the confirm overlay first and starts only on a yes. It refuses the
// command while another one runs.
func (model *Model) handleCommand(msg views.CommandMsg) tea.Cmd {
	if model.refuseWhileRunning() {
		return nil
	}
	question, ok := model.commands.Question(msg)
	if !ok {
		return model.startCommand(msg)
	}
	answer := make(chan bool, 1)
	model.push(newConfirmOverlay(question, answer, model.help, model.styles))
	return func() tea.Msg {
		if <-answer {
			return commandConfirmedMsg{msg: msg}
		}
		return nil
	}
}

// handleCommandConfirmed starts a command the user confirmed. It refuses the
// command while another one runs.
func (model *Model) handleCommandConfirmed(msg commandConfirmedMsg) tea.Cmd {
	if model.refuseWhileRunning() {
		return nil
	}
	return model.startCommand(msg.msg)
}

// refuseWhileRunning refuses a new command while another one runs and reports
// whether it refused it.
func (model *Model) refuseWhileRunning() bool {
	if model.running == nil {
		return false
	}
	model.running.isRefused = true
	return true
}

// startCommand starts a command and drops the verdict of the command before
// it.
func (model *Model) startCommand(msg views.CommandMsg) tea.Cmd {
	command, run := model.commands.Start(msg)
	model.running = &runningCommand{command: command, step: firstStepText}
	model.verdict = nil
	return tea.Batch(run, model.spinner.Tick)
}

// handleProgress shows the steps and the finished pieces of work of the
// running command and logs its warnings and errors. It skips the result lines,
// because the verdict of the command carries its outcome. It drops an event
// while no command runs.
func (model *Model) handleProgress(msg actions.ProgressMsg) {
	if model.running == nil {
		return
	}
	for _, line := range report.Lines(msg.Event, nameTask) {
		if line.Kind == report.KindResult {
			continue
		}
		switch line.Level {
		case report.LevelStep, report.LevelSuccess:
			model.running.step = line.Text
			model.running.isRefused = false
		case report.LevelWarning, report.LevelError:
			model.running.warningCount++
			model.events.add(eventLogEntry{at: time.Now(), command: model.running.command, level: line.Level, text: line.Text})
		}
	}
}

// handleCommandDone shows the verdict of the command that ended and refreshes
// the feeds it touched. The verdict of a command that worked or skipped clears
// after its lifetime. The error of a command that failed goes to the event log
// too, and the bar adds the hint of the command to it.
func (model *Model) handleCommandDone(msg actions.DoneMsg) tea.Cmd {
	warningCount := 0
	if model.running != nil {
		warningCount = model.running.warningCount
	}
	model.running = nil
	model.verdictCount++
	refresh := model.feedRunner.Refresh(msg.Feeds)
	if msg.Err != nil {
		model.events.add(eventLogEntry{at: time.Now(), command: msg.Command, level: report.LevelError, text: msg.Err.Error()})
		text := msg.Err.Error()
		if msg.Hint != "" {
			text += hintSeparator + msg.Hint
		}
		model.verdict = &verdict{text: text, isError: true, warningCount: warningCount, number: model.verdictCount}
		return refresh
	}
	number := model.verdictCount
	model.verdict = &verdict{text: msg.Verdict, isSkipped: msg.IsSkipped, warningCount: warningCount, number: number}
	expire := model.startTimer(verdictLifetime, func(time.Time) tea.Msg { return verdictExpiredMsg{number: number} })
	return tea.Batch(refresh, expire)
}

func (model *Model) handleVerdictExpired(msg verdictExpiredMsg) {
	if model.verdict != nil && model.verdict.number == msg.number {
		model.verdict = nil
	}
}

// handleSpinnerTick turns the spinner while a command runs and stops it
// otherwise.
func (model *Model) handleSpinnerTick(msg spinner.TickMsg) tea.Cmd {
	if model.running == nil {
		return nil
	}
	var cmd tea.Cmd
	model.spinner, cmd = model.spinner.Update(msg)
	return cmd
}

// isErrorShown reports whether the bottom bar shows the error of a command.
func (model *Model) isErrorShown() bool {
	return model.running == nil && model.verdict != nil && model.verdict.isError
}

// renderCommandLine returns the line of the bottom bar about commands: the
// spinner and the latest step of the running command, the refusal of a second
// command or the verdict of the command that ran last. It is empty when there
// is none of them.
func (model *Model) renderCommandLine() string {
	switch {
	case model.running != nil && model.running.isRefused:
		return model.renderWarning(fmt.Sprintf(refusalFormat, model.running.command))
	case model.running != nil:
		return fmt.Sprintf(runningFormat, model.spinner.View(), model.running.command.Target, model.running.step)
	case model.verdict == nil:
		return ""
	}
	line := successVerdictPrefix + model.verdict.text
	lineStyle := model.styles.Success
	switch {
	case model.verdict.isError:
		line = errorVerdictPrefix + model.verdict.text
		lineStyle = model.styles.Error
	case model.verdict.isSkipped:
		line = skippedVerdictPrefix + model.verdict.text
		lineStyle = model.styles.Muted
	}
	if model.verdict.warningCount > 0 {
		line += fmt.Sprintf(warningCountFormat, formatWarningCount(model.verdict.warningCount))
	}
	return lineStyle.Render(line)
}

func formatWarningCount(count int) string {
	if count == 1 {
		return "1 warning"
	}
	return fmt.Sprintf("%d warnings", count)
}
