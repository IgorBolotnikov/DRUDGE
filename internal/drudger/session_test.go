package drudger

import (
	"strings"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

const (
	erroredResultEvent = `{"type":"result","subtype":"success","is_error":true,"num_turns":2,"duration_ms":4000,"total_cost_usd":0.02,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","result":"Could not build"}`
	cutOffResultEvent  = `{"type":"result","subtype":"error_max_turns","is_error":false,"num_turns":50,"duration_ms":900000,"total_cost_usd":1.5,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","result":"Ran out of turns"}`
)

// noExitFile stands for a run directory there the agent has not finished work.
const noExitFile = ""

func TestReadSessionReport(t *testing.T) {
	cases := []struct {
		name string
		// stream holds the event lines the agent has written. A nil stream
		// stands for a run directory with no stream file in it.
		stream []string
		// exit is what the exit file holds, or noExitFile when the run has not
		// finished.
		exit string
		// since is how long after the last write drudge looks at the run.
		since time.Duration

		want            SessionStatus
		wantExitCode    int
		wantSessionID   string
		wantResult      bool
		wantVendorClass task.VendorErrorClass
		wantErr         bool
	}{
		{
			name:          "an agent still writing events",
			stream:        []string{initEvent, assistantEvent},
			exit:          noExitFile,
			want:          StatusWorking,
			wantSessionID: sampleSessionID,
		},
		{
			name:   "an agent that has not written anything yet",
			exit:   noExitFile,
			want:   StatusWorking,
			stream: nil,
		},
		{
			name:   "an event stream that is still empty",
			stream: []string{},
			exit:   noExitFile,
			want:   StatusWorking,
		},
		{
			name:          "an agent that has gone quiet",
			stream:        []string{initEvent},
			exit:          noExitFile,
			since:         sessionStaleAfter + time.Minute,
			want:          StatusNeedsBabysitting,
			wantSessionID: sampleSessionID,
		},
		{
			name:   "a launch that never produced an event",
			exit:   noExitFile,
			since:  sessionStaleAfter + time.Minute,
			want:   StatusNeedsBabysitting,
			stream: nil,
		},
		{
			name:          "an agent that exited non-zero",
			stream:        []string{initEvent},
			exit:          "1\n",
			want:          StatusFuckedUp,
			wantExitCode:  1,
			wantSessionID: sampleSessionID,
		},
		{
			name:          "a terminal event flagged as an error",
			stream:        []string{initEvent, erroredResultEvent},
			exit:          "0\n",
			want:          StatusFuckedUp,
			wantSessionID: sampleSessionID,
			wantResult:    true,
		},
		{
			name:          "a terminal event that did not succeed",
			stream:        []string{initEvent, cutOffResultEvent},
			exit:          "0\n",
			want:          StatusFuckedUp,
			wantSessionID: sampleSessionID,
			wantResult:    true,
		},
		{
			name:          "an agent that exited cleanly without a terminal event",
			stream:        []string{initEvent},
			exit:          "0\n",
			want:          StatusFuckedUp,
			wantSessionID: sampleSessionID,
		},
		{
			name:          "a run that finished the work",
			stream:        []string{initEvent, assistantEvent, resultEvent},
			exit:          "0\n",
			want:          StatusGotShitDone,
			wantSessionID: sampleSessionID,
			wantResult:    true,
		},
		{
			name:            "a run the vendor refused on credentials",
			stream:          []string{initEvent, authRefusedEvent, authRefusedResultEvent},
			exit:            "1\n",
			want:            StatusNeverGotGoing,
			wantExitCode:    1,
			wantSessionID:   sampleSessionID,
			wantResult:      true,
			wantVendorClass: task.VendorErrorAuth,
		},
		{
			name:            "a run the vendor refused on a rate limit",
			stream:          []string{initEvent, rateLimitedEvent, rateLimitedResultEvent},
			exit:            "1\n",
			want:            StatusNeverGotGoing,
			wantExitCode:    1,
			wantSessionID:   sampleSessionID,
			wantResult:      true,
			wantVendorClass: task.VendorErrorRateLimit,
		},
		{
			name:            "a refusal the agent reported no code for",
			stream:          []string{initEvent, uncodedRefusalResultEvent},
			exit:            "1\n",
			want:            StatusNeverGotGoing,
			wantExitCode:    1,
			wantSessionID:   sampleSessionID,
			wantResult:      true,
			wantVendorClass: task.VendorErrorUnknown,
		},
		{
			name:          "a terminal event the vendor had no part in",
			stream:        []string{initEvent, erroredResultEvent},
			exit:          "1\n",
			want:          StatusFuckedUp,
			wantExitCode:  1,
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
					t.Fatalf("expected an error, got status %q", report.Status)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if report.Status != testCase.want {
				t.Errorf("expected status %q, got %q", testCase.want, report.Status)
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
			if vendorClassOf(report) != testCase.wantVendorClass {
				t.Errorf("expected vendor error class %q, got %q", testCase.wantVendorClass, vendorClassOf(report))
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
		// for a task with no run directory.
		stream []string
		exit   string

		requestedID task.TaskID
		want        SessionStatus
		wantErr     bool
	}{
		{
			name:   "a task with an agent on it",
			status: task.StatusInProgress,
			stream: []string{initEvent, assistantEvent},
			exit:   noExitFile,
			want:   StatusWorking,
		},
		{
			name:   "a task whose agent finished the work",
			status: task.StatusInProgress,
			stream: []string{initEvent, resultEvent},
			exit:   "0\n",
			want:   StatusGotShitDone,
		},
		{
			name:        "a task named by a prefix of its id",
			status:      task.StatusInProgress,
			stream:      []string{initEvent},
			exit:        noExitFile,
			requestedID: "task",
			want:        StatusWorking,
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
			if session.Report.Status != testCase.want {
				t.Errorf("expected status %q, got %q", testCase.want, session.Report.Status)
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

func TestDrudgerService_SessionStatus_RecordsTheOutcome(t *testing.T) {
	cases := []struct {
		name string
		// stream holds the event lines of the task's run.
		stream []string
		exit   string

		wantStatus     task.TaskStatus
		wantFailed     bool
		wantResult     string
		wantTurns      int
		wantDuration   time.Duration
		wantCostUSD    float64
		wantFinishedAt bool
		// wantSessionID is what the task carries afterwards. Only a recorded
		// outcome puts one there, since these tasks never went through a launch.
		wantSessionID string
	}{
		{
			name:           "a run that got the work done",
			stream:         []string{initEvent, assistantEvent, resultEvent},
			exit:           "0\n",
			wantStatus:     task.StatusDone,
			wantResult:     "Done",
			wantTurns:      3,
			wantDuration:   8664 * time.Millisecond,
			wantCostUSD:    0.0695,
			wantFinishedAt: true,
			wantSessionID:  sampleSessionID,
		},
		{
			name:           "a run the agent flagged as an error",
			stream:         []string{initEvent, erroredResultEvent},
			exit:           "0\n",
			wantStatus:     task.StatusFuckedUp,
			wantFailed:     true,
			wantResult:     "Could not build",
			wantTurns:      2,
			wantDuration:   4 * time.Second,
			wantCostUSD:    0.02,
			wantFinishedAt: true,
			wantSessionID:  sampleSessionID,
		},
		{
			name:           "a run that died without a terminal event",
			stream:         []string{initEvent},
			exit:           "1\n",
			wantStatus:     task.StatusFuckedUp,
			wantFinishedAt: true,
			wantSessionID:  sampleSessionID,
		},
		{
			name:       "a run that is still going",
			stream:     []string{initEvent, assistantEvent},
			exit:       noExitFile,
			wantStatus: task.StatusInProgress,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			tracked := runningTask()
			runDir := common.RunDir(workspace, string(tracked.ID))
			writeStream(t, runDir, testCase.stream...)
			if testCase.exit != noExitFile {
				writeExit(t, runDir, testCase.exit)
			}

			service := newTestService(tracked)
			if _, err := service.SessionStatus(testProjectSlug, tracked.ID); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			recorded, err := service.tasks.GetTask(testProjectSlug, tracked.ID)
			if err != nil {
				t.Fatalf("could not read the task back: %v", err)
			}

			if recorded.Status != testCase.wantStatus {
				t.Errorf("expected task status %q, got %q", testCase.wantStatus, recorded.Status)
			}
			if recorded.SessionFailed != testCase.wantFailed {
				t.Errorf("expected the error flag %v, got %v", testCase.wantFailed, recorded.SessionFailed)
			}
			if recorded.SessionResult != testCase.wantResult {
				t.Errorf("expected result %q, got %q", testCase.wantResult, recorded.SessionResult)
			}
			if recorded.SessionTurns != testCase.wantTurns {
				t.Errorf("expected %d turns, got %d", testCase.wantTurns, recorded.SessionTurns)
			}
			if recorded.SessionDuration != testCase.wantDuration {
				t.Errorf("expected duration %s, got %s", testCase.wantDuration, recorded.SessionDuration)
			}
			if recorded.SessionCostUSD != testCase.wantCostUSD {
				t.Errorf("expected cost %v, got %v", testCase.wantCostUSD, recorded.SessionCostUSD)
			}
			if recorded.FinishedAt.IsZero() == testCase.wantFinishedAt {
				t.Errorf("expected a finish time stamped %v, got %v", testCase.wantFinishedAt, recorded.FinishedAt)
			}
			if recorded.SessionID != testCase.wantSessionID {
				t.Errorf("expected session id %q, got %q", testCase.wantSessionID, recorded.SessionID)
			}
		})
	}
}

func TestDrudgerService_SessionStatus_RecordsTheOutcomeOnce(t *testing.T) {
	workspace := setupWorkspace(t)
	tracked := runningTask()
	runDir := common.RunDir(workspace, string(tracked.ID))
	writeStream(t, runDir, initEvent, resultEvent)
	writeExit(t, runDir, "0\n")

	service := newTestService(tracked)
	if _, err := service.SessionStatus(testProjectSlug, tracked.ID); err != nil {
		t.Fatalf("unexpected error on the first check: %v", err)
	}

	first, err := service.tasks.GetTask(testProjectSlug, tracked.ID)
	if err != nil {
		t.Fatalf("could not read the task back: %v", err)
	}
	recorded := *first

	if _, err := service.SessionStatus(testProjectSlug, tracked.ID); err != nil {
		t.Fatalf("unexpected error on the second check: %v", err)
	}

	second, err := service.tasks.GetTask(testProjectSlug, tracked.ID)
	if err != nil {
		t.Fatalf("could not read the task back: %v", err)
	}
	if second.FinishedAt != recorded.FinishedAt {
		t.Errorf("expected the finish time %s to stand, got %s", recorded.FinishedAt, second.FinishedAt)
	}
	if second.Status != recorded.Status {
		t.Errorf("expected the status %q to stand, got %q", recorded.Status, second.Status)
	}
}

// runningTask is a task an agent has already been put on.
func runningTask() *task.Task {
	tracked := todoTask()
	tracked.Status = task.StatusInProgress
	tracked.StartedAt = time.Now().UTC()
	return tracked
}

// vendorClassOf reads the refusal class off a report, which carries none until
// the agent has written a terminal event.
func vendorClassOf(report SessionReport) task.VendorErrorClass {
	if report.Result == nil {
		return ""
	}
	return report.Result.VendorErrorClass
}

func TestDrudgerService_SessionStatus_RollsBackARefusedRun(t *testing.T) {
	cases := []struct {
		name string
		// stream holds the event lines of the task's run.
		stream []string

		wantStatus     task.TaskStatus
		wantVendorText string
		// wantVendorClass is empty for a run the vendor had no part in, which
		// is the case that pins the rollback to vendor refusals alone.
		wantVendorClass task.VendorErrorClass
		wantFinishedAt  bool
	}{
		{
			name:            "credentials the vendor would not take",
			stream:          []string{initEvent, authRefusedEvent, authRefusedResultEvent},
			wantStatus:      task.StatusTodo,
			wantVendorText:  authRefusedText,
			wantVendorClass: task.VendorErrorAuth,
		},
		{
			name:            "a rate limit",
			stream:          []string{initEvent, rateLimitedEvent, rateLimitedResultEvent},
			wantStatus:      task.StatusTodo,
			wantVendorText:  rateLimitedText,
			wantVendorClass: task.VendorErrorRateLimit,
		},
		{
			name:            "a refusal with no code to read",
			stream:          []string{initEvent, uncodedRefusalResultEvent},
			wantStatus:      task.StatusTodo,
			wantVendorText:  vendorOutageText,
			wantVendorClass: task.VendorErrorUnknown,
		},
		{
			name:           "an agent that failed on its own",
			stream:         []string{initEvent, erroredResultEvent},
			wantStatus:     task.StatusFuckedUp,
			wantFinishedAt: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			tracked := runningTask()
			runDir := common.RunDir(workspace, string(tracked.ID))
			writeStream(t, runDir, testCase.stream...)
			writeExit(t, runDir, "1\n")

			service := newTestService(tracked)

			var err error
			captureOutput(func() { _, err = service.SessionStatus(testProjectSlug, tracked.ID) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			recorded, err := service.tasks.GetTask(testProjectSlug, tracked.ID)
			if err != nil {
				t.Fatalf("could not read the task back: %v", err)
			}

			if recorded.Status != testCase.wantStatus {
				t.Errorf("expected task status %q, got %q", testCase.wantStatus, recorded.Status)
			}
			if recorded.VendorError != testCase.wantVendorText {
				t.Errorf("expected vendor error %q, got %q", testCase.wantVendorText, recorded.VendorError)
			}
			if recorded.VendorErrorClass != testCase.wantVendorClass {
				t.Errorf("expected vendor error class %q, got %q", testCase.wantVendorClass, recorded.VendorErrorClass)
			}
			if recorded.FinishedAt.IsZero() == testCase.wantFinishedAt {
				t.Errorf("expected a finish time stamped %v, got %v", testCase.wantFinishedAt, recorded.FinishedAt)
			}
		})
	}
}

func TestDrudgerService_SessionStatus_ARefusedRunRecordsNoWork(t *testing.T) {
	service, tracked := serviceWithAnAuthRefusal(t)
	recorded := checkRefusedTask(t, service, tracked.ID)

	if recorded.SessionFailed {
		t.Error("expected no error flag on a run that did no work")
	}
	if recorded.SessionResult != "" {
		t.Errorf("expected no session result, got %q", recorded.SessionResult)
	}
	if recorded.SessionTurns != 0 {
		t.Errorf("expected no turns recorded, got %d", recorded.SessionTurns)
	}
	if recorded.SessionDuration != 0 {
		t.Errorf("expected no duration recorded, got %s", recorded.SessionDuration)
	}
	if recorded.SessionCostUSD != 0 {
		t.Errorf("expected no cost recorded, got %v", recorded.SessionCostUSD)
	}
}

func TestDrudgerService_SessionStatus_RecordsTheSameRefusalOnEveryCheck(t *testing.T) {
	service, tracked := serviceWithAnAuthRefusal(t)

	first := checkRefusedTask(t, service, tracked.ID)
	second := checkRefusedTask(t, service, tracked.ID)

	if first.Status != second.Status {
		t.Errorf("expected the status %q to stand, got %q", first.Status, second.Status)
	}
	if first.VendorError != second.VendorError {
		t.Errorf("expected the vendor error %q to stand, got %q", first.VendorError, second.VendorError)
	}
	if first.VendorErrorClass != second.VendorErrorClass {
		t.Errorf("expected the vendor error class %q to stand, got %q", first.VendorErrorClass, second.VendorErrorClass)
	}
	if !second.FinishedAt.IsZero() {
		t.Errorf("expected no finish time on a run that never got going, got %v", second.FinishedAt)
	}
}

// serviceWithAnAuthRefusal sets up a task whose run the vendor turned away on
// credentials, with the whole finished run in its run directory.
func serviceWithAnAuthRefusal(t *testing.T) (*testService, *task.Task) {
	t.Helper()

	workspace := setupWorkspace(t)
	tracked := runningTask()
	runDir := common.RunDir(workspace, string(tracked.ID))
	writeStream(t, runDir, initEvent, authRefusedEvent, authRefusedResultEvent)
	writeExit(t, runDir, "1\n")

	return newTestService(tracked), tracked
}

// checkRefusedTask reports on a refused Session and reads back what the check
// wrote on the task.
func checkRefusedTask(t *testing.T, service *testService, taskID task.TaskID) task.Task {
	t.Helper()

	var session *TaskSession
	var err error
	captureOutput(func() { session, err = service.SessionStatus(testProjectSlug, taskID) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Report.Status != StatusNeverGotGoing {
		t.Fatalf("expected status %q, got %q", StatusNeverGotGoing, session.Report.Status)
	}

	recorded, err := service.tasks.GetTask(testProjectSlug, taskID)
	if err != nil {
		t.Fatalf("could not read the task back: %v", err)
	}
	return *recorded
}

func TestDrudgerService_SessionStatus_AnAuthRefusalNamesTheSbxCredentials(t *testing.T) {
	service, tracked := serviceWithAnAuthRefusal(t)

	var err error
	output := captureOutput(func() { _, err = service.SessionStatus(testProjectSlug, tracked.ID) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, wanted := range []string{"sbx", authRefusedText, string(task.StatusTodo)} {
		if !strings.Contains(output, wanted) {
			t.Errorf("expected the report to mention %q, got:\n%s", wanted, output)
		}
	}
}
