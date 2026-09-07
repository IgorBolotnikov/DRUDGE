package cmd

import (
	"fmt"
	"strconv"
	"time"

	"drudge/internal/common"
	"drudge/internal/drudger"
)

const (
	// sessionLabelWidth lines up the values of a status report, counting the
	// colon that follows every label.
	sessionLabelWidth = 11

	// notReportedLabel stands for a fact the agent has not written yet.
	notReportedLabel = "not reported yet"
)

// printSessionStatus prints what the run directory of a task says about the
// Session working on it.
func printSessionStatus(log *common.Logger, session *drudger.TaskSession) {
	report := session.Report

	lines := []string{
		fmt.Sprintf("Task [%s] %s", session.Task.ID, session.Task.Title),
		sessionLine("Session", string(report.Status)),
		sessionLine("Session id", orNotReported(report.SessionID)),
		sessionLine("Last write", formatAgo(report.LastWrite, time.Now().UTC())),
		sessionLine("Run dir", report.RunDir),
	}

	if report.Finished() {
		lines = append(lines, sessionLine("Exit code", strconv.Itoa(report.ExitCode)))
	}
	if result := report.Result; result != nil {
		lines = append(lines,
			sessionLine("Turns", strconv.Itoa(result.NumTurns)),
			sessionLine("Duration", result.Duration.Round(time.Second).String()),
			sessionLine("Cost", fmt.Sprintf("$%.4f", result.CostUSD)),
		)
		if result.VendorErrorClass != "" {
			lines = append(lines, sessionLine("Refused", string(result.VendorErrorClass)))
		}
		if result.Text != "" {
			lines = append(lines, "", "The agent said:", "", result.Text)
		}
	}

	for _, line := range lines {
		// The line is already formatted and may hold a percent sign.
		log.Info("%s", line)
	}
}

func sessionLine(label string, value string) string {
	return fmt.Sprintf("%s%-*s %s", listIndent, sessionLabelWidth, label+":", value)
}

func orNotReported(value string) string {
	if value == "" {
		return notReportedLabel
	}
	return value
}
