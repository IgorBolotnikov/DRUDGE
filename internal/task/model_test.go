package task

import (
	"testing"
	"time"
)

func TestTask_RunState(t *testing.T) {
	started := time.Date(2025, 3, 4, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name         string
		task         Task
		wantHasRun   bool
		wantRefused  bool
		wantFinished bool
		wantReported bool
	}{
		{
			name: "a task no agent has had",
			task: Task{Status: StatusTodo},
		},
		{
			name:       "a task an agent is working on",
			task:       Task{Status: StatusInProgress, StartedAt: started},
			wantHasRun: true,
		},
		{
			name: "a run the agent reported on",
			task: Task{
				Status:        StatusDone,
				StartedAt:     started,
				FinishedAt:    started.Add(time.Minute),
				SessionTurns:  3,
				SessionResult: "fixed the login",
			},
			wantHasRun:   true,
			wantFinished: true,
			wantReported: true,
		},
		{
			name: "a run killed with its Drudger",
			task: Task{
				Status:     StatusFuckedUp,
				StartedAt:  started,
				FinishedAt: started.Add(time.Minute),
			},
			wantHasRun:   true,
			wantFinished: true,
		},
		{
			name: "a run the vendor refused",
			task: Task{
				Status:           StatusTodo,
				StartedAt:        started,
				VendorError:      "OAuth session expired",
				VendorErrorClass: VendorErrorAuth,
			},
			wantHasRun:  true,
			wantRefused: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.task.HasRun(); got != testCase.wantHasRun {
				t.Errorf("expected HasRun %v, got %v", testCase.wantHasRun, got)
			}
			if got := testCase.task.WasRefused(); got != testCase.wantRefused {
				t.Errorf("expected WasRefused %v, got %v", testCase.wantRefused, got)
			}
			if got := testCase.task.RunFinished(); got != testCase.wantFinished {
				t.Errorf("expected RunFinished %v, got %v", testCase.wantFinished, got)
			}
			if got := testCase.task.AgentReported(); got != testCase.wantReported {
				t.Errorf("expected AgentReported %v, got %v", testCase.wantReported, got)
			}
		})
	}
}

func TestTask_StartRun_ClearsTheLastRun(t *testing.T) {
	taskToRun := Task{
		Status:           StatusFuckedUp,
		StartedAt:        time.Date(2025, 3, 4, 9, 0, 0, 0, time.UTC),
		FinishedAt:       time.Date(2025, 3, 4, 9, 5, 0, 0, time.UTC),
		SessionFailed:    true,
		SessionResult:    "could not find the login form",
		SessionTurns:     7,
		SessionDuration:  time.Minute,
		SessionCostUSD:   0.42,
		VendorError:      "OAuth session expired",
		VendorErrorClass: VendorErrorAuth,
	}

	startedAt := time.Date(2025, 3, 5, 11, 0, 0, 0, time.UTC)
	taskToRun.StartRun(startedAt, "ebe60e03")

	if taskToRun.Status != StatusInProgress {
		t.Errorf("expected status %q, got %q", StatusInProgress, taskToRun.Status)
	}
	if !taskToRun.HasRun() {
		t.Error("expected the task to count as run")
	}
	if taskToRun.RunFinished() {
		t.Error("expected the new run to be unfinished")
	}
	if taskToRun.AgentReported() {
		t.Error("expected what the previous agent reported to be cleared")
	}
	if taskToRun.WasRefused() {
		t.Error("expected the vendor refusal of the previous run to be cleared")
	}
	if taskToRun.SessionResult != "" || taskToRun.SessionTurns != 0 || taskToRun.SessionCostUSD != 0 {
		t.Errorf("expected what the previous run reported to be cleared, got %+v", taskToRun)
	}
}
