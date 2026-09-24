package cmd

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
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
	workLabel      = "Work"
	blockedByLabel = "Blocked by"
	parentLabel    = "Parent"
	childrenLabel  = "Children"
)

// What a task report prints in place of a field with nothing in it.
const (
	noneLabel               = "none"
	neverRunLabel           = "never handed to an agent"
	nothingReportedYetLabel = "nothing reported yet"
	endedUnreportedLabel    = "ended, the agent reported nothing"
	committedNothingLabel   = "the agent committed nothing"
	missingTaskLabel        = "no such task"
)

// relatedTaskLine lays out one task a task report links to, the status padded
// to the longest status there is.
const relatedTaskLine = "%s%s  %-11s  %s"

const (
	yesLabel = "yes"
	noLabel  = "no"
)

// printTask prints everything drudge knows about one task: what it asks for,
// where it stands, what blocks it, what it belongs to, what belongs to it and
// what its last run left behind. The description prints in full.
func printTask(log *common.Logger, taskToShow *task.Task, blockers []task.Blocker, unmerged map[task.TaskID][]drudger.UnmergedWork, family task.Family, now time.Time) {
	lines := []string{
		fmt.Sprintf("Task [%s] %s", taskToShow.ID, taskToShow.Title),
		taskLine(statusLabel, string(taskToShow.Status)),
		taskLine(ticketLabel, orNone(taskToShow.TicketID)),
		taskLine(createdLabel, formatMoment(taskToShow.CreatedAt, now)),
		taskLine(startedLabel, formatMoment(taskToShow.StartedAt, now)),
		taskLine(finishedLabel, formatMoment(taskToShow.FinishedAt, now)),
	}
	lines = append(lines, blockerLines(blockers, unmerged)...)
	lines = append(lines, familyLines(family)...)
	lines = append(lines,
		"",
		"Description:",
		"",
		orNone(taskToShow.Description),
		"",
	)
	lines = append(lines, taskRunLines(taskToShow)...)
	if taskToShow.HasRun() {
		lines = append(lines, "")
		lines = append(lines, taskWorkLines(taskToShow)...)
	}

	for _, line := range lines {
		// The line is already formatted and may hold a percent sign.
		log.Info("%s", line)
	}
}

// blockerLines lists the tasks a task waits for, one line per blocker, with the
// unmerged work of a blocker under it. A task blocked by nothing gets no lines.
func blockerLines(blockers []task.Blocker, unmerged map[task.TaskID][]drudger.UnmergedWork) []string {
	if len(blockers) == 0 {
		return nil
	}

	lines := []string{"", blockedByLabel + ":"}
	for _, blocker := range blockers {
		lines = append(lines, relatedLine(blocker.ID, blocker.Task))
		for _, line := range drudger.FormatUnmergedWork(unmerged[blocker.ID]) {
			lines = append(lines, listIndent+listIndent+line)
		}
	}
	return lines
}

// familyLines lists the task a task belongs to and the tasks belonging to it,
// each under a heading of its own. A section with no task in it gets no lines.
func familyLines(family task.Family) []string {
	var lines []string
	if family.ParentID != "" {
		lines = append(lines, "", parentLabel+":", relatedLine(family.ParentID, family.Parent))
	}
	if len(family.Children) > 0 {
		lines = append(lines, "", childrenLabel+":")
		for _, child := range family.Children {
			lines = append(lines, relatedLine(child.ID, child))
		}
	}
	return lines
}

// relatedLine renders one task a task report links to. related is nil when id
// names no stored task.
func relatedLine(id task.TaskID, related *task.Task) string {
	if related == nil {
		return listIndent + task.ShortID(id) + "  " + missingTaskLabel
	}
	return fmt.Sprintf(relatedTaskLine, listIndent, task.ShortID(id), related.Status, related.Title)
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
			taskLine(flaggedLabel, yesOrNo(taskToShow.HasSessionFailed)),
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

// taskWorkLines reports where the work of the last run is, one line per
// repository. A run drudge has not closed out yet lists the branch it was
// handed, with no range to read it by.
func taskWorkLines(taskToShow *task.Task) []string {
	heading := workLabel + ":"

	if len(taskToShow.Landings) == 0 {
		return []string{heading, listIndent + committedNothingLabel}
	}

	lines := []string{heading}
	for _, repository := range slices.Sorted(maps.Keys(taskToShow.Landings)) {
		lines = append(lines, taskLine(repository, formatLanding(taskToShow.Landings[repository])))
	}
	return lines
}

// formatLanding renders the branch the work of one repository is on, and the
// range holding it once the run has closed out.
func formatLanding(landing task.Landing) string {
	if landing.Commits == 0 {
		return landing.Branch
	}
	return fmt.Sprintf(
		"%s (%s, %s..%s)",
		landing.Branch, git.FormatCommitCount(landing.Commits), git.ShortSHA(landing.Base), git.ShortSHA(landing.Head),
	)
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

func yesOrNo(isYes bool) string {
	if isYes {
		return yesLabel
	}
	return noLabel
}
