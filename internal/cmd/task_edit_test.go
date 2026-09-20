package cmd

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"drudge/internal/task"
)

func TestParseTaskEditArgs(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantTaskID  task.TaskID
		wantChanges task.EditTaskDto
		wantErr     bool
		// wantErrText is a fragment the error must carry, checked when set.
		wantErrText string
	}{
		{
			name:        "a new title",
			args:        []string{"abc123", "--title", "Fix logout"},
			wantTaskID:  "abc123",
			wantChanges: task.EditTaskDto{Title: pointerTo("Fix logout")},
		},
		{
			name:        "a new description",
			args:        []string{"abc123", "--description", "SSO logs nobody out"},
			wantTaskID:  "abc123",
			wantChanges: task.EditTaskDto{Description: pointerTo("SSO logs nobody out")},
		},
		{
			name:        "a new ticket",
			args:        []string{"abc123", "--ticket", "R-004-01"},
			wantTaskID:  "abc123",
			wantChanges: task.EditTaskDto{TicketID: pointerTo("R-004-01")},
		},
		{
			name:        "a ticket cleared",
			args:        []string{"abc123", "--ticket", ""},
			wantTaskID:  "abc123",
			wantChanges: task.EditTaskDto{TicketID: pointerTo("")},
		},
		{
			name:        "a new status",
			args:        []string{"abc123", "--status", "todo"},
			wantTaskID:  "abc123",
			wantChanges: task.EditTaskDto{Status: pointerTo(task.StatusTodo)},
		},
		{
			name:        "the force flag",
			args:        []string{"abc123", "--status", "done", "--force"},
			wantTaskID:  "abc123",
			wantChanges: task.EditTaskDto{Status: pointerTo(task.StatusDone), AllowsManagedStatus: true},
		},
		{
			name:        "the short force flag",
			args:        []string{"abc123", "--status", "done", "-f"},
			wantTaskID:  "abc123",
			wantChanges: task.EditTaskDto{Status: pointerTo(task.StatusDone), AllowsManagedStatus: true},
		},
		{
			name:        "the task ID after the flags",
			args:        []string{"--title", "Fix logout", "abc123"},
			wantTaskID:  "abc123",
			wantChanges: task.EditTaskDto{Title: pointerTo("Fix logout")},
		},
		{
			name:       "every editable field at once",
			args:       []string{"abc123", "--title", "Fix logout", "--description", "SSO logs nobody out", "--ticket", "R-004-01", "--status", "todo"},
			wantTaskID: "abc123",
			wantChanges: task.EditTaskDto{
				Title:       pointerTo("Fix logout"),
				Description: pointerTo("SSO logs nobody out"),
				TicketID:    pointerTo("R-004-01"),
				Status:      pointerTo(task.StatusTodo),
			},
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: true,
		},
		{
			name:        "no task ID",
			args:        []string{"--title", "Fix logout"},
			wantErr:     true,
			wantErrText: "task ID is required",
		},
		{
			name:        "no field to change",
			args:        []string{"abc123"},
			wantErr:     true,
			wantErrText: taskEditUsage,
		},
		{
			name:        "a flag with no value",
			args:        []string{"abc123", "--title"},
			wantErr:     true,
			wantErrText: titleFlag,
		},
		{
			name:    "an unknown flag",
			args:    []string{"abc123", "--assignee", "nobody"},
			wantErr: true,
		},
		{
			name:        "a second task ID",
			args:        []string{"abc123", "def456", "--title", "Fix logout"},
			wantErr:     true,
			wantErrText: "drg task edit",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			taskID, changes, err := parseTaskEditArgs(testCase.args)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got task ID %q and changes %+v", taskID, changes)
				}
				if testCase.wantErrText != "" && !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Errorf("expected the error to name %q, got %q", testCase.wantErrText, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if taskID != testCase.wantTaskID {
				t.Errorf("expected task ID %q, got %q", testCase.wantTaskID, taskID)
			}
			if !reflect.DeepEqual(changes, testCase.wantChanges) {
				t.Errorf("expected %s, got %s", showChanges(testCase.wantChanges), showChanges(changes))
			}
		})
	}
}

func pointerTo[Value any](value Value) *Value {
	return &value
}

// showChanges renders a set of changes with the values behind its pointers.
func showChanges(changes task.EditTaskDto) string {
	fields := []string{
		showFlag(titleFlag, changes.Title),
		showFlag(descriptionFlag, changes.Description),
		showFlag(ticketFlag, changes.TicketID),
		showFlag(statusFlag, changes.Status),
		fmt.Sprintf("%s %v", forceFlag, changes.AllowsManagedStatus),
	}
	return strings.Join(fields, ", ")
}

// showFlag renders one field of an edit, naming a field the edit leaves alone.
func showFlag[Value ~string](flag string, value *Value) string {
	if value == nil {
		return flag + " unset"
	}
	return flag + " " + strconv.Quote(string(*value))
}
