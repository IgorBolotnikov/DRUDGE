package cmd

import (
	"strings"
	"testing"

	"drudge/internal/task"
)

func TestParseTaskRemoveArgs(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantTaskID task.TaskID
		wantForce  bool
		wantErr    bool
		// wantErrText is a fragment the error must carry, checked when set.
		wantErrText string
	}{
		{
			name:       "a task ID",
			args:       []string{"abc123"},
			wantTaskID: "abc123",
		},
		{
			name:       "the force flag",
			args:       []string{"abc123", "--force"},
			wantTaskID: "abc123",
			wantForce:  true,
		},
		{
			name:       "the short force flag",
			args:       []string{"abc123", "-f"},
			wantTaskID: "abc123",
			wantForce:  true,
		},
		{
			name:       "the task ID after the flag",
			args:       []string{"--force", "abc123"},
			wantTaskID: "abc123",
			wantForce:  true,
		},
		{
			name:        "no arguments",
			args:        nil,
			wantErr:     true,
			wantErrText: "task ID is required",
		},
		{
			name:    "an unknown flag",
			args:    []string{"abc123", "--recursive"},
			wantErr: true,
		},
		{
			name:        "a second task ID",
			args:        []string{"abc123", "def456"},
			wantErr:     true,
			wantErrText: "drg task rm",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			taskID, isForced, err := parseTaskRemoveArgs(testCase.args)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got task ID %q and force %v", taskID, isForced)
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
			if isForced != testCase.wantForce {
				t.Errorf("expected force %v, got %v", testCase.wantForce, isForced)
			}
		})
	}
}
