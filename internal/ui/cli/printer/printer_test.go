package printer

import (
	"io"
	"os"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func TestPrinter(t *testing.T) {
	sampleTask := &task.Task{ID: "3f9a1c2e-0b1d-4c2e-9f3a-1c2e0b1d4c2e", Title: "Add retry to uploader"}

	cases := []struct {
		name       string
		env        map[string]string
		print      func(out *Printer)
		wantStdout string
		wantStderr string
	}{
		{
			name: "lines inside a group are indented and the result sits at column 0",
			print: func(out *Printer) {
				out.Header("Setting up")
				out.Done("Created a")
				out.Skip("b already exists")
				out.Result("Set up")
			},
			wantStdout: "Setting up\n  ✓ Created a\n  · b already exists\n✓ Set up\n",
		},
		{
			name: "lines with no open group sit at column 0",
			print: func(out *Printer) {
				out.Done("Created a")
				out.Skip("b already exists")
				out.Warn("c is broken")
			},
			wantStdout: "✓ Created a\n· b already exists\n",
			wantStderr: "! c is broken\n",
		},
		{
			name: "a second header gets one blank line before it",
			print: func(out *Printer) {
				out.Header("First")
				out.Done("a")
				out.Result("First done")
				out.Header("Second")
				out.Done("b")
				out.Result("Second done")
			},
			wantStdout: "First\n  ✓ a\n✓ First done\n\nSecond\n  ✓ b\n✓ Second done\n",
		},
		{
			name: "a header right after a header gets one blank line before it",
			print: func(out *Printer) {
				out.Header("First")
				out.Header("Second")
			},
			wantStdout: "First\n\nSecond\n",
		},
		{
			name: "a header after a line with no open group gets a blank line before it",
			print: func(out *Printer) {
				out.Done("a")
				out.Header("Next")
			},
			wantStdout: "✓ a\n\nNext\n",
		},
		{
			name: "a warning counts as printed before a header",
			print: func(out *Printer) {
				out.Warn("a is broken")
				out.Header("Next")
			},
			wantStdout: "\nNext\n",
			wantStderr: "! a is broken\n",
		},
		{
			name:       "a lone header has no blank line around it",
			print:      func(out *Printer) { out.Header("Only") },
			wantStdout: "Only\n",
		},
		{
			name: "a warning inside a group goes to stderr with the indent",
			print: func(out *Printer) {
				out.Header("Fetching")
				out.Warn("Could not fetch %s", "main")
				out.Result("Fetched")
			},
			wantStdout: "Fetching\n✓ Fetched\n",
			wantStderr: "  ! Could not fetch main\n",
		},
		{
			name: "a step ends with an ellipsis",
			print: func(out *Printer) {
				out.Step("Fetching %s", "main")
				out.Header("Running")
				out.Step("Starting the agent")
			},
			wantStdout: "› Fetching main…\n\nRunning\n  › Starting the agent…\n",
		},
		{
			name: "a detail sits one level deeper than the lines around it",
			print: func(out *Printer) {
				out.Detail("sbx: %s", "pulling")
				out.Header("Running")
				out.Step("Creating sandbox")
				out.Detail("sbx: %s", "pulling")
			},
			wantStdout: "    sbx: pulling\n\nRunning\n  › Creating sandbox…\n      sbx: pulling\n",
		},
		{
			name:       "a detail loses the ANSI codes of its text",
			print:      func(out *Printer) { out.Detail("sbx: %s", "\x1b[2K\x1b[32mpulling\x1b[0m image") },
			wantStdout: "    sbx: pulling image\n",
		},
		{
			name: "the values of consecutive fields line up",
			print: func(out *Printer) {
				out.Header("Running")
				out.Result("Started")
				out.Field("Branch", "drudge/3f9a-add-retry")
				out.Field("Run dir", ".drudge/runs/3f9a1c2e")
				out.Flush()
			},
			wantStdout: "Running\n✓ Started\n    Branch   drudge/3f9a-add-retry\n    Run dir  .drudge/runs/3f9a1c2e\n",
		},
		{
			name: "the next line ends a run of fields",
			print: func(out *Printer) {
				out.Field("A", "1")
				out.Field("Longer", "2")
				out.Done("Next")
				out.Field("Branch", "3")
				out.Warn("Broken")
			},
			wantStdout: "    A       1\n    Longer  2\n✓ Next\n    Branch  3\n",
			wantStderr: "! Broken\n",
		},
		{
			name: "fields in an open group sit at the detail indent of the group",
			print: func(out *Printer) {
				out.Header("Running")
				out.Field("Branch", "main")
				out.Result("Started")
			},
			wantStdout: "Running\n      Branch  main\n✓ Started\n",
		},
		{
			name:       "a field with no value prints its label alone",
			print:      func(out *Printer) { out.Field("Commands", ""); out.Flush() },
			wantStdout: "    Commands\n",
		},
		{
			name: "a block in a group sits at the field indent with a blank line around it",
			print: func(out *Printer) {
				out.Header("Dry run")
				out.Field("Prompt from", "built-in prompt")
				out.Block("Fix login\n\nSSO is broken\n")
				out.Field("Commands", "")
				out.Block(`"sbx" "ls"`)
				out.SkipResult("Nothing ran")
			},
			wantStdout: "Dry run\n" +
				"      Prompt from  built-in prompt\n" +
				"\n" +
				"      Fix login\n" +
				"\n" +
				"      SSO is broken\n" +
				"\n" +
				"      Commands\n" +
				"\n" +
				`      "sbx" "ls"` + "\n" +
				"· Nothing ran\n",
		},
		{
			name:       "a block printed first has no blank line before or after it",
			print:      func(out *Printer) { out.Block("a\nb") },
			wantStdout: "    a\n    b\n",
		},
		{
			name: "two blocks in a row get one blank line between them",
			print: func(out *Printer) {
				out.Block("a")
				out.Block("b")
			},
			wantStdout: "    a\n\n    b\n",
		},
		{
			name: "a header after a block gets one blank line before it",
			print: func(out *Printer) {
				out.Block("a")
				out.Header("Next")
			},
			wantStdout: "    a\n\nNext\n",
		},
		{
			name: "a warning after a block gets the blank line before it",
			print: func(out *Printer) {
				out.Block("a")
				out.Warn("b is broken")
				out.Done("c")
			},
			wantStdout: "    a\n\n✓ c\n",
			wantStderr: "! b is broken\n",
		},
		{
			name: "an empty block prints nothing",
			print: func(out *Printer) {
				out.Done("a")
				out.Block("\n")
				out.Done("b")
			},
			wantStdout: "✓ a\n✓ b\n",
		},
		{
			name: "a warning result and a failed result close the group at column 0",
			print: func(out *Printer) {
				out.Header("Recording")
				out.Done("a")
				out.ResultWarn("Needs babysitting")
				out.Header("Recording")
				out.ResultFailed("Fucked up")
			},
			wantStdout: "Recording\n  ✓ a\n! Needs babysitting\n\nRecording\n✗ Fucked up\n",
		},
		{
			name: "fields after a warning result sit at the detail indent",
			print: func(out *Printer) {
				out.Header("Recording")
				out.ResultWarn("Refused")
				out.Field("Advice", "Log in again")
				out.Flush()
			},
			wantStdout: "Recording\n! Refused\n    Advice  Log in again\n",
		},
		{
			name: "a view closes the group and gets one blank line before it",
			print: func(out *Printer) {
				out.Header("Recording")
				out.Result("Done")
				out.Field("Unblocked", "b")
				out.View([]string{"Task a", "  Session:  100%"})
			},
			wantStdout: "Recording\n✓ Done\n    Unblocked  b\n\nTask a\n  Session:  100%\n",
		},
		{
			name: "a view drops blank lines at its start and its end and prints a run of them as one",
			print: func(out *Printer) {
				out.Done("a")
				out.View([]string{"", "Task a", "", "", "Description", ""})
			},
			wantStdout: "✓ a\n\nTask a\n\nDescription\n",
		},
		{
			name:       "a view printed first has no blank line at its start",
			print:      func(out *Printer) { out.View([]string{"", "Task a"}) },
			wantStdout: "Task a\n",
		},
		{
			name:       "a view printed first has no blank line before it",
			print:      func(out *Printer) { out.View([]string{"Task a"}) },
			wantStdout: "Task a\n",
		},
		{
			name:       "a task reads as its short id, two spaces and its title",
			print:      func(out *Printer) { out.Done("Picked %s", out.Task(sampleTask)) },
			wantStdout: "✓ Picked 3f9a1c2e  Add retry to uploader\n",
		},
		{
			name:       "forced color makes the short id of a task bold",
			env:        map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print:      func(out *Printer) { out.Done("Picked %s", out.Task(sampleTask)) },
			wantStdout: "\x1b[32m✓\x1b[0m Picked \x1b[1m3f9a1c2e\x1b[0m  Add retry to uploader\n",
		},
		{
			name: "forced color mutes the step glyph, the detail and the field label",
			env:  map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print: func(out *Printer) {
				out.Step("Creating sandbox")
				out.Detail("sbx: \x1b[31mpulling\x1b[0m")
				out.Field("Branch", "main")
				out.Flush()
			},
			wantStdout: "\x1b[2m›\x1b[0m Creating sandbox…\n" +
				"    \x1b[2msbx: pulling\x1b[0m\n" +
				"    \x1b[2mBranch\x1b[0m  main\n",
		},
		{
			name: "forced color leaves a block in the default color",
			env:  map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print: func(out *Printer) {
				out.Field("Commands", "")
				out.Block(`"sbx" "ls"`)
			},
			wantStdout: "    \x1b[2mCommands\x1b[0m\n\n" + `    "sbx" "ls"` + "\n",
		},
		{
			name:       "forced color mutes the glyph of a skipped result",
			env:        map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print:      func(out *Printer) { out.SkipResult("Nothing ran") },
			wantStdout: "\x1b[2m·\x1b[0m Nothing ran\n",
		},
		{
			name:       "forced color makes the header bold",
			env:        map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print:      func(out *Printer) { out.Header("Setting up") },
			wantStdout: "\x1b[1mSetting up\x1b[0m\n",
		},
		{
			name: "forced color paints the glyphs in their roles",
			env:  map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print: func(out *Printer) {
				out.Header("Setting up")
				out.Done("Created a")
				out.Skip("b already exists")
				out.Warn("c is broken")
				out.Result("Set up")
			},
			wantStdout: "\x1b[1mSetting up\x1b[0m\n" +
				"  \x1b[32m✓\x1b[0m Created a\n" +
				"  \x1b[2m·\x1b[0m b already exists\n" +
				"\x1b[32m✓\x1b[0m Set up\n",
			wantStderr: "  \x1b[33m!\x1b[0m c is broken\n",
		},
		{
			name: "forced color paints the warning and the failed result in their roles",
			env:  map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print: func(out *Printer) {
				out.ResultWarn("Needs babysitting")
				out.ResultFailed("Fucked up")
			},
			wantStdout: "\x1b[33m!\x1b[0m Needs babysitting\n" +
				"\x1b[31m✗\x1b[0m Fucked up\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			for name, value := range testCase.env {
				t.Setenv(name, value)
			}
			palette := theme.NewTheme(theme.DefaultTheme())
			out := NewPrinter(common.NewLogger("", common.Labels{Error: palette.ErrorLabel(), Warn: palette.WarnLabel()}), palette)

			var stdout string
			stderr := captureStderr(func() {
				stdout = captureOutput(func() { testCase.print(out) })
			})

			if stdout != testCase.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, testCase.wantStdout)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, testCase.wantStderr)
			}
		})
	}
}

func captureOutput(f func()) string {
	orig := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer
	f()
	writer.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(reader)
	return string(out)
}

func captureStderr(f func()) string {
	orig := os.Stderr
	reader, writer, _ := os.Pipe()
	os.Stderr = writer
	f()
	writer.Close()
	os.Stderr = orig
	out, _ := io.ReadAll(reader)
	return string(out)
}
