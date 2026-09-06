package drudger

import (
	"strings"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

// More sample terminal events, each breaking exactly one of the rules that
// make a finished run a success.
const (
	erroredResultEvent = `{"type":"result","subtype":"success","is_error":true,"num_turns":2,"duration_ms":4000,"total_cost_usd":0.02,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","result":"Could not build"}`
	cutOffResultEvent  = `{"type":"result","subtype":"error_max_turns","is_error":false,"num_turns":50,"duration_ms":900000,"total_cost_usd":1.5,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","result":"Ran out of turns"}`
)

// noExitFile stands for a run directory the agent has not finished in, since
// an exit file always holds something.
const noExitFile = ""

func TestReadSessionReport(t *testing.T) {
	cases := []struct {
		name string
		// stream holds the event lines the agent has written. A nil stream
		// stands for a run directory with no stream file in it at all.
		stream []string
		// exit is what the exit file holds, or noExitFile when the run has not
		// finished.
		exit string
		// since is how long after the last write drudge looks at the run.
		since time.Duration

		want          SessionVerdict
		wantExitCode  int
		wantSessionID string
		wantResult    bool
		wantErr       bool
	}{
		{
			name:          "an agent still writing events",
			stream:        []string{initEvent, assistantEvent},
			exit:          noExitFile,
			want:          VerdictWorking,
			wantSessionID: sampleSessionID,
		},
		{
			name:   "an agent that has not written anything yet",
			exit:   noExitFile,
			want:   VerdictWorking,
			stream: nil,
		},
		{
			name:   "an event stream that is still empty",
			stream: []string{},
			exit:   noExitFile,
			want:   VerdictWorking,
		},
		{
			name:          "an agent that has gone quiet",
			stream:        []string{initEvent},
			exit:          noExitFile,
			since:         sessionStaleAfter + time.Minute,
			want:          VerdictNeedsBabysitting,
			wantSessionID: sampleSessionID,
		},
		{
			name:   "a launch that never produced an event",
			exit:   noExitFile,
			since:  sessionStaleAfter + time.Minute,
			want:   VerdictNeedsBabysitting,
			stream: nil,
		},
		{
			name:          "an agent that exited non-zero",
			stream:        []string{initEvent},
			exit:          "1\n",
			want:          VerdictFuckedUp,
			wantExitCode:  1,
			wantSessionID: sampleSessionID,
		},
		{
			name:          "a terminal event flagged as an error",
			stream:        []string{initEvent, erroredResultEvent},
			exit:          "0\n",
			want:          VerdictFuckedUp,
			wantSessionID: sampleSessionID,
			wantResult:    true,
		},
		{
			name:          "a terminal event that did not succeed",
			stream:        []string{initEvent, cutOffResultEvent},
			exit:          "0\n",
			want:          VerdictFuckedUp,
			wantSessionID: sampleSessionID,
			wantResult:    true,
		},
		{
			name:          "an agent that exited cleanly without a terminal event",
			stream:        []string{initEvent},
			exit:          "0\n",
			want:          VerdictFuckedUp,
			wantSessionID: sampleSessionID,
		},
		{
			name:          "a run that finished the work",
			stream:        []string{initEvent, assistantEvent, resultEvent},
			exit:          "0\n",
			want:          VerdictGotShitDone,
			wantSessionID: sampleSessionID,
			wantResult:    true,
		},
		{
			name:    "an exit file holding something that is not an exit code",
			stream:  []string{initEvent},
			exit:    "killed\n",
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runDir := t.TempDir()
			if testCase.stream != nil {
				writeStream(t, runDir, testCase.stream...)
			}
			if testCase.exit != noExitFile {
				writeExit(t, runDir, testCase.exit)
			}

			report, err := readSessionReport(runDir, time.Now().UTC().Add(testCase.since))

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got verdict %q", report.Verdict)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if report.Verdict != testCase.want {
				t.Errorf("expected verdict %q, got %q", testCase.want, report.Verdict)
			}
			if report.ExitCode != testCase.wantExitCode {
				t.Errorf("expected exit code %d, got %d", testCase.wantExitCode, report.ExitCode)
			}
			if report.SessionID != testCase.wantSessionID {
				t.Errorf("expected session id %q, got %q", testCase.wantSessionID, report.SessionID)
			}
			if (report.Result != nil) != testCase.wantResult {
				t.Errorf("expected a terminal event recorded %v, got %+v", testCase.wantResult, report.Result)
			}
			if report.RunDir != runDir {
				t.Errorf("expected run directory %q, got %q", runDir, report.RunDir)
			}
			if report.LastWrite.IsZero() {
				t.Error("expected the last write to be stamped")
			}
		})
	}
}

func TestReadSessionReport_MissingRunDirectory(t *testing.T) {
	missing := common.RunDir(t.TempDir(), "never-run")

	if _, err := readSessionReport(missing, time.Now().UTC()); err == nil {
		t.Fatal("expected an error for a run directory that is not there")
	}
}

func TestReadSessionReport_KeepsTheFactsOfATerminalEvent(t *testing.T) {
	runDir := t.TempDir()
	writeStream(t, runDir, initEvent, resultEvent)
	writeExit(t, runDir, "0\n")

	report, err := readSessionReport(runDir, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := SessionResult{
		Subtype:  "success",
		NumTurns: 3,
		Duration: 8664 * time.Millisecond,
		CostUSD:  0.0695,
		Text:     "Done",
	}
	if report.Result == nil {
		t.Fatal("expected the terminal event to be recorded")
	}
	if *report.Result != want {
		t.Errorf("expected result %+v, got %+v", want, *report.Result)
	}
}

func TestDrudgerService_SessionStatus(t *testing.T) {
	cases := []struct {
		name string
		// status is what the task carries when drudge is asked about it.
		status task.TaskStatus
		// stream holds the event lines of the task's run. A nil stream stands
		// for a task with no run directory at all.
		stream []string
		exit   string

		requestedID task.TaskID
		want        SessionVerdict
		wantErr     bool
	}{
		{
			name:   "a task with an agent on it",
			status: task.StatusInProgress,
			stream: []string{initEvent, assistantEvent},
			exit:   noExitFile,
			want:   VerdictWorking,
		},
		{
			name:   "a task whose agent finished the work",
			status: task.StatusInProgress,
			stream: []string{initEvent, resultEvent},
			exit:   "0\n",
			want:   VerdictGotShitDone,
		},
		{
			name:        "a task named by a prefix of its id",
			status:      task.StatusInProgress,
			stream:      []string{initEvent},
			exit:        noExitFile,
			requestedID: "task",
			want:        VerdictWorking,
		},
		{
			name:    "a task that was never run",
			status:  task.StatusTodo,
			wantErr: true,
		},
		{
			name:        "a task id nothing carries",
			status:      task.StatusTodo,
			requestedID: "nonesuch",
			wantErr:     true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			tracked := todoTask()
			tracked.Status = testCase.status
			if testCase.stream != nil {
				runDir := common.RunDir(workspace, string(tracked.ID))
				writeStream(t, runDir, testCase.stream...)
				if testCase.exit != noExitFile {
					writeExit(t, runDir, testCase.exit)
				}
			}

			service := newTestService(tracked)
			requestedID := testCase.requestedID
			if requestedID == "" {
				requestedID = tracked.ID
			}

			session, err := service.SessionStatus(testProjectSlug, requestedID)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", session)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if session.Task.ID != tracked.ID {
				t.Errorf("expected task %q, got %q", tracked.ID, session.Task.ID)
			}
			if session.Report.Verdict != testCase.want {
				t.Errorf("expected verdict %q, got %q", testCase.want, session.Report.Verdict)
			}
		})
	}
}

func TestDrudgerService_SessionStatus_NeverRunTaskNamesItsStatus(t *testing.T) {
	setupWorkspace(t)
	tracked := todoTask()
	service := newTestService(tracked)

	_, err := service.SessionStatus(testProjectSlug, tracked.ID)
	if err == nil {
		t.Fatal("expected an error for a task that was never run")
	}
	if !strings.Contains(err.Error(), string(task.StatusTodo)) {
		t.Errorf("expected the error to name the task status, got %q", err)
	}
}

// writeExit puts the exit file of a finished run in a run directory, creating
// the directory if the test has not.
func writeExit(t *testing.T, runDir string, contents string) {
	t.Helper()
	ensureRunDir(t, runDir)
	if err := common.WriteFile(common.RunExitPath(runDir), contents); err != nil {
		t.Fatalf("could not write the exit file: %v", err)
	}
}
