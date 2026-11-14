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

// runRound is one round of a run read in parts: what the run gets written,
// then what a read with the cursor of the round before finds.
type runRound struct {
	// isRerun prepares the run again before the writes.
	isRerun bool
	stream  string
	stderr  string

	wantStreamLines   []string
	wantStderrLines   []string
	wantStreamMissing bool
	wantReset         bool
}

func TestDrudgerService_RunStepsFrom(t *testing.T) {
	cases := []struct {
		name string
		// isLegacyRun stands for a run directory with no run ID file.
		isLegacyRun bool
		rounds      []runRound
	}{
		{
			name: "complete lines appended between reads",
			rounds: []runRound{
				{stream: initEvent + "\n", stderr: "first\n", wantStreamLines: []string{initEvent}, wantStderrLines: []string{"first"}},
				{stream: assistantEvent + "\n" + resultEvent + "\n", wantStreamLines: []string{assistantEvent, resultEvent}},
				{},
			},
		},
		{
			name: "a line split across two writes",
			rounds: []runRound{
				{stream: initEvent + "\n" + resultEvent[:20], stderr: "half", wantStreamLines: []string{initEvent}},
				{stream: resultEvent[20:], stderr: " a line"},
				{stream: "\n", stderr: "\n", wantStreamLines: []string{resultEvent}, wantStderrLines: []string{"half a line"}},
			},
		},
		{
			name: "stderr growing on its own",
			rounds: []runRound{
				{stderr: "first\n", wantStderrLines: []string{"first"}, wantStreamMissing: true},
				{stderr: "second\nthird\n", wantStderrLines: []string{"second", "third"}, wantStreamMissing: true},
				{stream: initEvent + "\n", wantStreamLines: []string{initEvent}},
			},
		},
		{
			name: "a rerun starts over",
			rounds: []runRound{
				{stream: initEvent + "\n" + resultEvent + "\n", stderr: "old\n", wantStreamLines: []string{initEvent, resultEvent}, wantStderrLines: []string{"old"}},
				{isRerun: true, stream: initEvent + "\n", stderr: "new\n", wantStreamLines: []string{initEvent}, wantStderrLines: []string{"new"}, wantReset: true},
				{stream: assistantEvent + "\n", wantStreamLines: []string{assistantEvent}},
			},
		},
		{
			name: "a rerun before the first read is no reset",
			rounds: []runRound{
				{isRerun: true, stream: initEvent + "\n", wantStreamLines: []string{initEvent}},
			},
		},
		{
			name:        "a run without a run ID file",
			isLegacyRun: true,
			rounds: []runRound{
				{stream: initEvent + "\n", wantStreamLines: []string{initEvent}},
				{stream: resultEvent + "\n", stderr: "late\n", wantStreamLines: []string{resultEvent}, wantStderrLines: []string{"late"}},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupProjectDir(t)
			tracked := todoTask()
			service := newTestService(tracked)
			if testCase.isLegacyRun {
				service.runs.write(tracked.ID, common.RunPromptName, "prompt")
			} else if err := service.runs.PrepareRun(tracked.ID, "prompt", HarnessClaudeCode); err != nil {
				t.Fatalf("PrepareRun: %v", err)
			}

			var cursor RunCursor
			for index, round := range testCase.rounds {
				if round.isRerun {
					if err := service.runs.PrepareRun(tracked.ID, "prompt", HarnessClaudeCode); err != nil {
						t.Fatalf("round %d: PrepareRun: %v", index, err)
					}
				}
				if round.stream != "" {
					service.runs.appendFile(tracked.ID, common.RunStreamName, round.stream)
				}
				if round.stderr != "" {
					service.runs.appendFile(tracked.ID, common.RunStderrName, round.stderr)
				}

				read, err := service.RunStepsFrom(testProjectSlug, tracked.ID, cursor)
				if err != nil {
					t.Fatalf("round %d: unexpected error: %v", index, err)
				}
				if !read.HasRun {
					t.Fatalf("round %d: expected the task to have a run", index)
				}
				if read.IsReset != round.wantReset {
					t.Errorf("round %d: expected reset %v, got %v", index, round.wantReset, read.IsReset)
				}
				if read.IsStreamMissing != round.wantStreamMissing {
					t.Errorf("round %d: expected the stream missing %v, got %v", index, round.wantStreamMissing, read.IsStreamMissing)
				}
				var wantSteps []Step
				for _, line := range round.wantStreamLines {
					wantSteps = append(wantSteps, fakeStreamReader{}.Decode(line)...)
				}
				if !reflect.DeepEqual(read.Steps, wantSteps) {
					t.Errorf("round %d: expected steps %+v, got %+v", index, wantSteps, read.Steps)
				}
				if !reflect.DeepEqual(read.Stderr, round.wantStderrLines) {
					t.Errorf("round %d: expected stderr lines %q, got %q", index, round.wantStderrLines, read.Stderr)
				}
				cursor = read.Next
			}
		})
	}
}

func TestDrudgerService_RunStepsFrom_NoRun(t *testing.T) {
	setupProjectDir(t)
	tracked := todoTask()
	service := newTestService(tracked)

	read, err := service.RunStepsFrom(testProjectSlug, tracked.ID, RunCursor{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if read.HasRun {
		t.Error("expected a task with no run")
	}
	if read.Task.ID != tracked.ID {
		t.Errorf("expected task %q, got %q", tracked.ID, read.Task.ID)
	}
	if len(read.Steps) != 0 || len(read.Stderr) != 0 {
		t.Errorf("expected nothing read, got steps %+v and stderr %q", read.Steps, read.Stderr)
	}
	if !reflect.DeepEqual(read.Next, RunCursor{}) {
		t.Errorf("expected the zero cursor, got %+v", read.Next)
	}
}
