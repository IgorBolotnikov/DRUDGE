package cmd

import (
	"strings"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/drudger"
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

func TestParseTaskStatusArgs(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantTaskID task.TaskID
		wantErr    bool
	}{
		{
			name:       "task ID only",
			args:       []string{"abc123"},
			wantTaskID: "abc123",
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"abc123", "--watch"},
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
			taskID, err := parseTaskStatusArgs(testCase.args)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got task ID %q", taskID)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if taskID != testCase.wantTaskID {
				t.Errorf("expected task ID %q, got %q", testCase.wantTaskID, taskID)
			}
		})
	}
}

func TestPrintSessionStatus(t *testing.T) {
	cases := []struct {
		name   string
		report drudger.SessionReport
		want   []string
	}{
		{
			name: "an agent still working",
			report: drudger.SessionReport{
				Verdict:   drudger.VerdictWorking,
				RunDir:    "/tmp/run",
				SessionID: "ebe60e03",
				LastWrite: time.Now(),
			},
			want: []string{"working", "ebe60e03", "/tmp/run", "just now"},
		},
		{
			name: "an agent that has not reported its session yet",
			report: drudger.SessionReport{
				Verdict:   drudger.VerdictNeedsBabysitting,
				LastWrite: time.Now().Add(-2 * time.Hour),
			},
			want: []string{"needs babysitting", notReportedLabel, "2h ago"},
		},
		{
			name: "a finished run",
			report: drudger.SessionReport{
				Verdict:   drudger.VerdictGotShitDone,
				LastWrite: time.Now(),
				ExitCode:  0,
				Result: &drudger.SessionResult{
					Subtype:  "success",
					NumTurns: 3,
					Duration: 8664 * time.Millisecond,
					CostUSD:  0.0695,
					Text:     "Fixed the login",
				},
			},
			want: []string{"got shit done", "3", "9s", "$0.0695", "Fixed the login"},
		},
		{
			name: "a run that fell over",
			report: drudger.SessionReport{
				Verdict:   drudger.VerdictFuckedUp,
				LastWrite: time.Now(),
				ExitCode:  137,
			},
			want: []string{"fucked up", "137"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := &drudger.TaskSession{
				Task:   &task.Task{ID: "abc123", Title: "Fix login"},
				Report: testCase.report,
			}
			log := common.NewLogger("")

			out := captureOutput(func() { printSessionStatus(log, session) })

			for _, want := range append(testCase.want, "abc123", "Fix login") {
				if !strings.Contains(out, want) {
					t.Errorf("expected the report to mention %q, got:\n%s", want, out)
				}
			}
		})
	}
}

func TestRunTask_UnknownSubcommand(t *testing.T) {
	if err := runTask([]string{"frobnicate"}); err == nil {
		t.Fatal("expected an error for an unknown task subcommand")
	}
}
