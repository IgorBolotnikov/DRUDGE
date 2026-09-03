package cmd

import (
	"testing"

	"drudge/internal/task"
)

func TestParseTaskRunArgs(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantTaskID task.TaskID
		wantDryRun bool
		wantErr    bool
	}{
		{
			name:       "task ID only",
			args:       []string{"abc123"},
			wantTaskID: "abc123",
		},
		{
			name:       "task ID with dry run flag",
			args:       []string{"abc123", "--dry-run"},
			wantTaskID: "abc123",
			wantDryRun: true,
		},
		{
			name:       "dry run flag before the task ID",
			args:       []string{"--dry-run", "abc123"},
			wantTaskID: "abc123",
			wantDryRun: true,
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "dry run flag without a task ID",
			args:    []string{"--dry-run"},
			wantErr: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"abc123", "--detached"},
			wantErr: true,
		},
		{
			name:    "second task ID",
			args:    []string{"abc123", "def456"},
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			taskID, dryRun, err := parseTaskRunArgs(testCase.args)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got task ID %q and dry run %v", taskID, dryRun)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if taskID != testCase.wantTaskID {
				t.Errorf("expected task ID %q, got %q", testCase.wantTaskID, taskID)
			}
			if dryRun != testCase.wantDryRun {
				t.Errorf("expected dry run %v, got %v", testCase.wantDryRun, dryRun)
			}
		})
	}
}

func TestRunTask_UnknownSubcommand(t *testing.T) {
	if err := runTask([]string{"frobnicate"}); err == nil {
		t.Fatal("expected an error for an unknown task subcommand")
	}
}
