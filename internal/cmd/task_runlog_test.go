package cmd

import (
	"testing"

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
			out := newPrinter(newThemedLogger(palette), palette)

			var stdout string
			stderr := captureStderr(func() {
				stdout = captureOutput(func() {
					if testCase.warning != "" {
						out.warn("%s", testCase.warning)
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
