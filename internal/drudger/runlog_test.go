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

func TestDrudgerService_RunSteps(t *testing.T) {
	cases := []struct {
		name string
		// hasRun stands for a task whose run directory exists. stream and
		// stderr are only written when it is set.
		hasRun bool
		// configured is the harness in the config, claude-code when empty.
		// recorded is the harness the run holds, none when empty.
		configured Harness
		recorded   Harness
		stream     []string
		stderr     string

		requestedID       task.TaskID
		wantHarness       Harness
		wantDecoded       bool
		wantStreamMissing bool
		wantStderrLines   []string
		wantErr           bool
	}{
		{
			name:            "a run whose stream the decoder turns into steps",
			hasRun:          true,
			recorded:        HarnessClaudeCode,
			stream:          []string{initEvent, resultEvent},
			stderr:          "warning: 100% of nothing",
			wantHarness:     HarnessClaudeCode,
			wantDecoded:     true,
			wantStderrLines: []string{"warning: 100% of nothing"},
		},
		{
			name:              "a run whose agent has not written a stream yet",
			hasRun:            true,
			recorded:          HarnessClaudeCode,
			wantHarness:       HarnessClaudeCode,
			wantDecoded:       true,
			wantStreamMissing: true,
		},
		{
			name:        "a run decodes with the harness it recorded over the configured one",
			hasRun:      true,
			configured:  HarnessOpencode,
			recorded:    HarnessClaudeCode,
			stream:      []string{initEvent},
			wantHarness: HarnessClaudeCode,
			wantDecoded: true,
		},
		{
			name:        "a run with no recorded harness decodes with the configured one",
			hasRun:      true,
			stream:      []string{initEvent},
			wantHarness: HarnessClaudeCode,
			wantDecoded: true,
		},
		{
			name:        "a run whose recorded harness has no decoder gives raw steps",
			hasRun:      true,
			recorded:    HarnessOpencode,
			stream:      []string{initEvent, resultEvent},
			wantHarness: HarnessOpencode,
		},
		{
			name:        "a run with no recorded harness and a configured harness with no decoder gives raw steps",
			hasRun:      true,
			configured:  HarnessOpencode,
			stream:      []string{initEvent},
			wantHarness: HarnessOpencode,
		},
		{
			name:    "a task that was never run",
			wantErr: true,
		},
		{
			name:        "a task id nothing carries",
			hasRun:      true,
			requestedID: "nonesuch",
			wantErr:     true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupProjectDir(t)
			tracked := todoTask()

			settings := testSettings()
			if testCase.configured != "" {
				settings.Harness = testCase.configured
			}
			service := newTestServiceWith(settings, &fakeCommandRunner{}, tracked)
			if testCase.hasRun {
				service.runs.write(tracked.ID, common.RunPromptName, "prompt")
				if testCase.recorded != "" {
					service.runs.write(tracked.ID, common.RunHarnessName, string(testCase.recorded))
				}
				if testCase.stream != nil {
					service.runs.writeStream(tracked.ID, testCase.stream...)
				}
				if testCase.stderr != "" {
					service.runs.writeStderr(tracked.ID, testCase.stderr)
				}
			}
			requestedID := testCase.requestedID
			if requestedID == "" {
				requestedID = tracked.ID
			}

			runSteps, err := service.RunSteps(testProjectSlug, requestedID)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", runSteps)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if runSteps.Task.ID != tracked.ID {
				t.Errorf("expected task %q, got %q", tracked.ID, runSteps.Task.ID)
			}
			if runSteps.Harness != testCase.wantHarness {
				t.Errorf("expected harness %q, got %q", testCase.wantHarness, runSteps.Harness)
			}
			if runSteps.IsDecoded != testCase.wantDecoded {
				t.Errorf("expected decoded %v, got %v", testCase.wantDecoded, runSteps.IsDecoded)
			}
			if runSteps.IsStreamMissing != testCase.wantStreamMissing {
				t.Errorf("expected the stream missing %v, got %v", testCase.wantStreamMissing, runSteps.IsStreamMissing)
			}

			var wantSteps []Step
			for _, line := range testCase.stream {
				if testCase.wantDecoded {
					wantSteps = append(wantSteps, fakeStreamReader{}.Decode(line)...)
				} else {
					wantSteps = append(wantSteps, Step{Kind: StepUnrecognized, Raw: line})
				}
			}
			if !reflect.DeepEqual(runSteps.Steps, wantSteps) {
				t.Errorf("expected steps %+v, got %+v", wantSteps, runSteps.Steps)
			}
			if !reflect.DeepEqual(runSteps.Stderr, testCase.wantStderrLines) {
				t.Errorf("expected stderr lines %q, got %q", testCase.wantStderrLines, runSteps.Stderr)
			}
		})
	}
}
