package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const (
	// sessionLabelWidth fits the longest label of a status report.
	sessionLabelWidth = 11

	// notReportedLabel stands for a fact the agent has not written yet.
	notReportedLabel = "not reported yet"
)

// sessionStatusRoles maps a Session status to the theme role it prints in.
var sessionStatusRoles = map[drudger.SessionStatus]string{
	drudger.StatusWorking:          theme.RoleInfo,
	drudger.StatusNeedsBabysitting: theme.RoleWarning,
	drudger.StatusFuckedUp:         theme.RoleError,
	drudger.StatusGotShitDone:      theme.RoleSuccess,
	drudger.StatusNeverGotGoing:    theme.RoleError,
}

// printSessionStatus prints what the run directory of a task says about the
// Session working on it.
func printSessionStatus(log *common.Logger, session *drudger.TaskSession) {
	report := session.Report
	statusColor := loadRoleColor(log, "Session status", sessionStatusRoles)

	lines := []string{
		fmt.Sprintf("Task [%s] %s", session.Task.ID, session.Task.Title),
		sessionLine("Session", statusColor(string(report.Status))),
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
	return labelledLine(label, sessionLabelWidth, value)
}

func orNotReported(value string) string {
	if value == "" {
		return notReportedLabel
	}
	return value
}
