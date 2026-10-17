package cmd

import (
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func TestPrintRunLogs(t *testing.T) {
	ranTask := &task.Task{ID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", Title: "Fix login"}
	events := drudger.RunLog{Name: "events", Path: ".drudge/runs/006684e3/events.jsonl", Lines: []string{`{"type":"result"}`, "100% done"}}

	cases := []struct {
		name string
		// warning prints before the logs.
		warning    string
		logs       []drudger.RunLog
		want       string
		wantStderr string
	}{
		{
			name: "a log with lines and a log not written yet",
			logs: []drudger.RunLog{events, {Name: "stderr", Path: ".drudge/runs/006684e3/stderr.log", IsMissing: true}},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				"\n" +
				"events (.drudge/runs/006684e3/events.jsonl):\n" +
				"\n" +
				"{\n  \"type\": \"result\"\n}\n" +
				"100% done\n" +
				"\n" +
				"stderr (.drudge/runs/006684e3/stderr.log):\n" +
				"\n" +
				notWrittenYetLabel + "\n",
		},
		{
			name: "an empty log",
			logs: []drudger.RunLog{{Name: "stderr", Path: ".drudge/runs/006684e3/stderr.log"}},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				"\n" +
				"stderr (.drudge/runs/006684e3/stderr.log):\n" +
				"\n" +
				emptyLogLabel + "\n",
		},
		{
			name: "a log ending in a blank line",
			logs: []drudger.RunLog{{Name: "stderr", Path: ".drudge/runs/006684e3/stderr.log", Lines: []string{"", "failed", ""}}},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				"\n" +
				"stderr (.drudge/runs/006684e3/stderr.log):\n" +
				"\n" +
				"failed\n",
		},
		{
			name:    "a warning printed before the logs",
			warning: "theme.json is broken",
			logs:    []drudger.RunLog{{Name: "stderr", Path: ".drudge/runs/006684e3/stderr.log"}},
			want: "\n" +
				"Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				"\n" +
				"stderr (.drudge/runs/006684e3/stderr.log):\n" +
				"\n" +
				emptyLogLabel + "\n",
			wantStderr: "! theme.json is broken\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			palette := theme.NewTheme(theme.DefaultTheme())
			out := printer.NewPrinter(newThemedLogger(palette), palette)

			var stdout string
			stderr := captureStderr(func() {
				stdout = captureOutput(func() {
					if testCase.warning != "" {
						out.Warn("%s", testCase.warning)
					}
					printRunLogs(out, &drudger.TaskRunLogs{Task: ranTask, Logs: testCase.logs})
				})
			})

			if stdout != testCase.want {
				t.Errorf("expected:\n%s\ngot:\n%s", testCase.want, stdout)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, testCase.wantStderr)
			}
		})
	}
}

func TestPrintRunSteps(t *testing.T) {
	ranTask := &task.Task{ID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", Title: "Fix login"}
	const runDir = ".drudge/runs/006684e3-dbe9-4316-8aba-8a67a8f01f8f"

	cases := []struct {
		name  string
		steps []drudger.Step
		// isStreamMissing stands for a run whose agent has not written the
		// event stream yet.
		isStreamMissing bool
		stderr          []string
		want            string
	}{
		{
			name: "a compact view of a run",
			steps: []drudger.Step{
				{Kind: drudger.StepInit, Summary: "session 02005590… claude-opus · /work/slot-1"},
				{Kind: drudger.StepToolCall, Tool: "Skill", Summary: "impl-ticket"},
				{Kind: drudger.StepToolCall, Tool: "Read", Summary: "internal/cmd/task_runlog.go"},
				{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go test ./internal/cmd/..."},
				{Kind: drudger.StepAgentText, Summary: "I'll fix the failing test by adding a case."},
				{Kind: drudger.StepToolCall, Tool: "Edit", Summary: "internal/cmd/task_runlog.go"},
				{Kind: drudger.StepFinalResult, Summary: "got shit done · 42 turns · 6m12s · $1.84"},
			},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				runDir + "\n" +
				"session 02005590… claude-opus · /work/slot-1\n" +
				"Skill   impl-ticket\n" +
				"Read    internal/cmd/task_runlog.go\n" +
				"Bash    go test ./internal/cmd/...\n" +
				"I'll fix the failing test by adding a case.\n" +
				"Edit    internal/cmd/task_runlog.go\n" +
				"got shit done · 42 turns · 6m12s · $1.84\n",
		},
		{
			name:  "an unknown tool shows its name only",
			steps: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "WebSearch"}},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				runDir + "\n" +
				"WebSearch\n",
		},
		{
			name:  "a tool error prints its summary alone",
			steps: []drudger.Step{{Kind: drudger.StepToolError, Summary: "Exit code 123…"}},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				runDir + "\n" +
				"Exit code 123…\n",
		},
		{
			name:  "an unrecognized line is printed as it is",
			steps: []drudger.Step{{Kind: drudger.StepUnrecognized, Raw: `{"type":"tool_progress"}`}},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				runDir + "\n" +
				`{"type":"tool_progress"}` + "\n",
		},
		{
			name:  "a fucked up final result",
			steps: []drudger.Step{{Kind: drudger.StepFinalResult, Summary: "fucked up · 5 turns · 2m5s · $0.50"}},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				runDir + "\n" +
				"fucked up · 5 turns · 2m5s · $0.50\n",
		},
		{
			name:            "a stream not written yet",
			isStreamMissing: true,
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				runDir + "\n" +
				notWrittenYetLabel + "\n",
		},
		{
			name:   "stderr with content prints under its heading",
			steps:  []drudger.Step{{Kind: drudger.StepFinalResult, Summary: "got shit done · 1 turns · 1s · $0.01"}},
			stderr: []string{"warning: 100% of nothing", "second line"},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				runDir + "\n" +
				"got shit done · 1 turns · 1s · $0.01\n" +
				"\n" +
				stderrHeading + "\n" +
				"warning: 100% of nothing\n" +
				"second line\n",
		},
		{
			name:  "empty stderr prints no heading",
			steps: []drudger.Step{{Kind: drudger.StepFinalResult, Summary: "got shit done · 1 turns · 1s · $0.01"}},
			want: "Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n" +
				runDir + "\n" +
				"got shit done · 1 turns · 1s · $0.01\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			palette := theme.NewTheme(theme.DefaultTheme())
			out := printer.NewPrinter(newThemedLogger(palette), palette)

			stdout := captureOutput(func() {
				printRunSteps(out, &drudger.TaskRunSteps{
					Task:            ranTask,
					RunDir:          runDir,
					Steps:           testCase.steps,
					IsStreamMissing: testCase.isStreamMissing,
					Stderr:          testCase.stderr,
				})
			})

			if stdout != testCase.want {
				t.Errorf("expected:\n%s\ngot:\n%s", testCase.want, stdout)
			}
		})
	}
}

func TestFormatLogLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "a JSON object",
			line: `{"type":"system","tools":["Bash"]}`,
			want: "{\n  \"type\": \"system\",\n  \"tools\": [\n    \"Bash\"\n  ]\n}",
		},
		{
			name: "a JSON object with whitespace around it",
			line: "  {\"type\":\"result\"}\r",
			want: "{\n  \"type\": \"result\"\n}",
		},
		{
			name: "plain text",
			line: "error: could not connect",
			want: "error: could not connect",
		},
		{
			name: "a line with a percent sign",
			line: "progress: 100%d done",
			want: "progress: 100%d done",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := formatLogLine(testCase.line); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
