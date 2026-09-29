package drudger

import (
	"reflect"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// noStderr stands for a run directory with no stderr log in it.
const noStderr = "none"

func TestDrudgerService_RunLogs(t *testing.T) {
	cases := []struct {
		name string
		// stream holds the event lines of the task's run. A nil stream stands
		// for a task with no run directory.
		stream []string
		stderr string

		requestedID       task.TaskID
		wantStreamLines   []string
		wantStderrLines   []string
		wantStderrMissing bool
		wantErr           bool
	}{
		{
			name:            "a run that wrote both logs",
			stream:          []string{initEvent, resultEvent},
			stderr:          "warning: 100% of nothing\nsecond line\n",
			wantStreamLines: []string{initEvent, resultEvent},
			wantStderrLines: []string{"warning: 100% of nothing", "second line"},
		},
		{
			name:              "a run with no stderr log yet",
			stream:            []string{initEvent},
			stderr:            noStderr,
			wantStreamLines:   []string{initEvent},
			wantStderrMissing: true,
		},
		{
			name:            "a task named by a prefix of its id",
			stream:          []string{initEvent},
			stderr:          "",
			requestedID:     "task",
			wantStreamLines: []string{initEvent},
		},
		{
			name:    "a task that was never run",
			wantErr: true,
		},
		{
			name:        "a task id nothing carries",
			stream:      []string{initEvent},
			stderr:      noStderr,
			requestedID: "nonesuch",
			wantErr:     true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupProjectDir(t)
			tracked := todoTask()

			service := newTestService(tracked)
			if testCase.stream != nil {
				service.runs.writeStream(tracked.ID, testCase.stream...)
				if testCase.stderr != noStderr {
					service.runs.writeStderr(tracked.ID, testCase.stderr)
				}
			}
			requestedID := testCase.requestedID
			if requestedID == "" {
				requestedID = tracked.ID
			}

			runLogs, err := service.RunLogs(testProjectSlug, requestedID)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", runLogs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if runLogs.Task.ID != tracked.ID {
				t.Errorf("expected task %q, got %q", tracked.ID, runLogs.Task.ID)
			}
			if len(runLogs.Logs) != 2 {
				t.Fatalf("expected the stream and the stderr log, got %+v", runLogs.Logs)
			}

			stream, stderr := runLogs.Logs[0], runLogs.Logs[1]
			if stream.Name != common.RunStreamName || stderr.Name != common.RunStderrName {
				t.Errorf("expected %s then %s, got %s then %s", common.RunStreamName, common.RunStderrName, stream.Name, stderr.Name)
			}
			if !reflect.DeepEqual(stream.Lines, testCase.wantStreamLines) {
				t.Errorf("expected stream lines %q, got %q", testCase.wantStreamLines, stream.Lines)
			}
			if !reflect.DeepEqual(stderr.Lines, testCase.wantStderrLines) {
				t.Errorf("expected stderr lines %q, got %q", testCase.wantStderrLines, stderr.Lines)
			}
			if stderr.IsMissing != testCase.wantStderrMissing {
				t.Errorf("expected the stderr log missing %v, got %v", testCase.wantStderrMissing, stderr.IsMissing)
			}
		})
	}
}
