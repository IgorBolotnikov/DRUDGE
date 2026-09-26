package cmd

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestTaskEditFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// files are written to the directory the flags are read in, by name.
		files       map[string]string
		stdin       string
		wantChanges task.EditTaskDto
		wantErr     bool
		// wantErrText is a fragment the error must carry, checked when set.
		wantErrText string
	}{
		{
			name:        "a new title",
			args:        []string{"--title", "Fix logout"},
			wantChanges: task.EditTaskDto{Title: pointerTo("Fix logout")},
		},
		{
			name:        "a new description",
			args:        []string{"--description", "SSO logs nobody out"},
			wantChanges: task.EditTaskDto{Description: pointerTo("SSO logs nobody out")},
		},
		{
			name:        "a new ticket",
			args:        []string{"--ticket", "R-004-01"},
			wantChanges: task.EditTaskDto{TicketID: pointerTo("R-004-01")},
		},
		{
			name:        "a ticket cleared",
			args:        []string{"--ticket", ""},
			wantChanges: task.EditTaskDto{TicketID: pointerTo("")},
		},
		{
			name:        "a new status",
			args:        []string{"--status", "todo"},
			wantChanges: task.EditTaskDto{Status: pointerTo(task.StatusTodo)},
		},
		{
			name:        "the force flag",
			args:        []string{"--status", "done", "--force"},
			wantChanges: task.EditTaskDto{Status: pointerTo(task.StatusDone), AllowsManagedStatus: true},
		},
		{
			name:        "the short force flag",
			args:        []string{"--status", "done", "-f"},
			wantChanges: task.EditTaskDto{Status: pointerTo(task.StatusDone), AllowsManagedStatus: true},
		},
		{
			name: "every editable field at once",
			args: []string{"--title", "Fix logout", "--description", "SSO logs nobody out", "--ticket", "R-004-01", "--status", "todo"},
			wantChanges: task.EditTaskDto{
				Title:       pointerTo("Fix logout"),
				Description: pointerTo("SSO logs nobody out"),
				TicketID:    pointerTo("R-004-01"),
				Status:      pointerTo(task.StatusTodo),
			},
		},
		{
			name:        "a list of blockers",
			args:        []string{"--blocked-by", "9c8d7e6f,1a2b3c4d"},
			wantChanges: task.EditTaskDto{BlockedBy: &[]task.TaskID{"9c8d7e6f", "1a2b3c4d"}},
		},
		{
			name:        "a list of blockers with spaces and an empty entry",
			args:        []string{"--blocked-by", " 9c8d7e6f, ,1a2b3c4d "},
			wantChanges: task.EditTaskDto{BlockedBy: &[]task.TaskID{"9c8d7e6f", "1a2b3c4d"}},
		},
		{
			name:        "the blockers cleared",
			args:        []string{"--blocked-by", ""},
			wantChanges: task.EditTaskDto{BlockedBy: &[]task.TaskID{}},
		},
		{
			name:        "a parent",
			args:        []string{"--parent", "9c8d7e6f"},
			wantChanges: task.EditTaskDto{ParentTaskID: pointerTo(task.TaskID("9c8d7e6f"))},
		},
		{
			name:        "the parent cleared",
			args:        []string{"--parent", ""},
			wantChanges: task.EditTaskDto{ParentTaskID: pointerTo(task.TaskID(""))},
		},
		{
			name:        "blockers to add",
			args:        []string{"--block", "9c8d7e6f,1a2b3c4d"},
			wantChanges: task.EditTaskDto{Block: &[]task.TaskID{"9c8d7e6f", "1a2b3c4d"}},
		},
		{
			name:        "blockers to remove",
			args:        []string{"--unblock", "9c8d7e6f,1a2b3c4d"},
			wantChanges: task.EditTaskDto{Unblock: &[]task.TaskID{"9c8d7e6f", "1a2b3c4d"}},
		},
		{
			name:        "the blocker list and blockers to add",
			args:        []string{"--blocked-by", "9c8d", "--block", "1a2b"},
			wantErr:     true,
			wantErrText: "--blocked-by and --block cannot be used together",
		},
		{
			name:        "the blocker list and blockers to remove",
			args:        []string{"--blocked-by", "9c8d", "--unblock", "1a2b"},
			wantErr:     true,
			wantErrText: "--blocked-by and --unblock cannot be used together",
		},
		{
			name:        "blockers to add and blockers to remove",
			args:        []string{"--block", "9c8d", "--unblock", "1a2b"},
			wantErr:     true,
			wantErrText: "--block and --unblock cannot be used together",
		},
		{
			name:        "all three blocker flags",
			args:        []string{"--unblock", "1a2b", "--block", "9c8d", "--blocked-by", "7e6d"},
			wantErr:     true,
			wantErrText: "--blocked-by, --block and --unblock cannot be used together",
		},
		{
			name:        "a description file",
			args:        []string{"--description-file", "description.md"},
			files:       map[string]string{"description.md": "SSO logs nobody out"},
			wantChanges: task.EditTaskDto{Description: pointerTo("SSO logs nobody out")},
		},
		{
			name:        "a description from stdin",
			args:        []string{"--description-file", "-"},
			stdin:       "SSO logs nobody out",
			wantChanges: task.EditTaskDto{Description: pointerTo("SSO logs nobody out")},
		},
		{
			name:        "a description ending in a newline",
			args:        []string{"--description-file", "-"},
			stdin:       "SSO logs nobody out\n\n",
			wantChanges: task.EditTaskDto{Description: pointerTo("SSO logs nobody out\n")},
		},
		{
			name:        "a description with a code block and dollar signs",
			args:        []string{"--description-file", "-"},
			stdin:       literalDescription + "\n",
			wantChanges: task.EditTaskDto{Description: pointerTo(literalDescription)},
		},
		{
			name:        "a description and a description file",
			args:        []string{"--description-file", "-", "--description", "SSO logs nobody out"},
			stdin:       "SSO logs nobody out",
			wantErr:     true,
			wantErrText: "--description and --description-file cannot be used together",
		},
		{
			name:        "a missing description file",
			args:        []string{"--description-file", "missing.md"},
			wantErr:     true,
			wantErrText: `"missing.md"`,
		},
		{
			name:        "an empty description file",
			args:        []string{"--description-file", "description.md"},
			files:       map[string]string{"description.md": ""},
			wantErr:     true,
			wantErrText: `"description.md" holds no description`,
		},
		{
			name:        "a whitespace-only description from stdin",
			args:        []string{"--description-file", "-"},
			stdin:       " \n\t\n",
			wantErr:     true,
			wantErrText: "stdin holds no description",
		},
		{
			name:        "no field to change",
			args:        nil,
			wantErr:     true,
			wantErrText: "nothing to change",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			writeFiles(t, testCase.files)

			flags := &taskEditFlags{}
			parseTestFlags(t, flags.declare, testCase.args)

			changes, err := flags.changes(strings.NewReader(testCase.stdin))

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got changes %+v", changes)
				}
				if testCase.wantErrText != "" && !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Errorf("expected the error to name %q, got %q", testCase.wantErrText, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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
		showFlag(flagLabel(titleFlagName), changes.Title),
		showFlag(flagLabel(descriptionFlagName), changes.Description),
		showFlag(flagLabel(ticketFlagName), changes.TicketID),
		showFlag(flagLabel(statusFlagName), changes.Status),
		showBlockers(flagLabel(blockedByFlagName), changes.BlockedBy),
		showBlockers(flagLabel(blockFlagName), changes.Block),
		showBlockers(flagLabel(unblockFlagName), changes.Unblock),
		fmt.Sprintf("%s %v", flagLabel(forceFlagName), changes.AllowsManagedStatus),
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

func showBlockers(flag string, ids *[]task.TaskID) string {
	if ids == nil {
		return flag + " unset"
	}
	return fmt.Sprintf("%s %q", flag, *ids)
}
