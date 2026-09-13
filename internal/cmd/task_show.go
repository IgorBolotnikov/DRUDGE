package cmd

import (
	"fmt"
	"strconv"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

const (
	// taskLabelWidth fits the longest label of a task report.
	taskLabelWidth = 12

	// momentLayout prints a timestamp. formatMoment converts to UTC first, so
	// the zone name renders as UTC.
	momentLayout = "2006-01-02 15:04 MST"
)

// Labels of a task report.
const (
	statusLabel    = "Status"
	ticketLabel    = "Ticket"
	createdLabel   = "Created"
	startedLabel   = "Started"
	finishedLabel  = "Finished"
	lastRunLabel   = "Last run"
	sessionIDLabel = "Session id"
	outcomeLabel   = "Outcome"
	turnsLabel     = "Turns"
	durationLabel  = "Duration"
	costLabel      = "Cost"
	flaggedLabel   = "Agent error"
	refusedLabel   = "Refused"
)

// What a task report prints in place of a field with nothing in it.
const (
	noneLabel               = "none"
	neverRunLabel           = "never handed to an agent"
	nothingReportedYetLabel = "nothing reported yet"
	endedUnreportedLabel    = "ended, the agent reported nothing"
)

const (
	yesLabel = "yes"
	noLabel  = "no"
)

// printTask prints everything drudge knows about one task: what it asks for,
// where it stands and what its last run left behind. The description prints in
// full.
func printTask(log *common.Logger, taskToShow *task.Task, now time.Time) {
	lines := []string{
		fmt.Sprintf("Task [%s] %s", taskToShow.ID, taskToShow.Title),
		taskLine(statusLabel, string(taskToShow.Status)),
		taskLine(ticketLabel, orNone(taskToShow.TicketID)),
		taskLine(createdLabel, formatMoment(taskToShow.CreatedAt, now)),
		taskLine(startedLabel, formatMoment(taskToShow.StartedAt, now)),
		taskLine(finishedLabel, formatMoment(taskToShow.FinishedAt, now)),
		"",
		"Description:",
		"",
		orNone(taskToShow.Description),
		"",
	}
	lines = append(lines, taskRunLines(taskToShow)...)

	for _, line := range lines {
		// The line is already formatted and may hold a percent sign.
		log.Info("%s", line)
	}
}

// taskRunLines reports what the last run left on a task. A task no agent has
// had carries a zero in every field these lines read, so it gets a single line
// under the heading.
func taskRunLines(taskToShow *task.Task) []string {
	heading := lastRunLabel + ":"

	if !taskToShow.HasRun() {
		return []string{heading, listIndent + neverRunLabel}
	}

	var detail []string
	if taskToShow.SessionID != "" {
		detail = append(detail, taskLine(sessionIDLabel, taskToShow.SessionID))
	}

	var said []string
	switch {
	case taskToShow.WasRefused():
		detail = append(detail, taskLine(refusedLabel, string(taskToShow.VendorErrorClass)))
		said = textBlock("The vendor said:", taskToShow.VendorError)
	case taskToShow.AgentReported():
		detail = append(detail,
			taskLine(turnsLabel, strconv.Itoa(taskToShow.SessionTurns)),
			taskLine(durationLabel, taskToShow.SessionDuration.Round(time.Second).String()),
			taskLine(costLabel, fmt.Sprintf("$%.4f", taskToShow.SessionCostUSD)),
			taskLine(flaggedLabel, yesOrNo(taskToShow.SessionFailed)),
		)
		said = textBlock("The agent said:", taskToShow.SessionResult)
	case taskToShow.RunFinished():
		detail = append(detail, taskLine(outcomeLabel, endedUnreportedLabel))
	default:
		detail = append(detail, taskLine(outcomeLabel, nothingReportedYetLabel))
	}

	lines := append([]string{heading}, detail...)
	return append(lines, said...)
}

func taskLine(label string, value string) string {
	return labelledLine(label, taskLabelWidth, value)
}

// textBlock renders a block of text under a heading of its own. An empty text
// renders nothing.
func textBlock(heading string, text string) []string {
	if text == "" {
		return nil
	}
	return []string{"", heading, "", text}
}

// formatMoment renders a timestamp with how long ago it was.
func formatMoment(moment time.Time, now time.Time) string {
	if moment.IsZero() {
		return neverLabel
	}
	return fmt.Sprintf("%s (%s)", moment.UTC().Format(momentLayout), formatAgo(moment, now))
}

func orNone(value string) string {
	if value == "" {
		return noneLabel
	}
	return value
}

func yesOrNo(value bool) string {
	if value {
		return yesLabel
	}
	return noLabel
}
