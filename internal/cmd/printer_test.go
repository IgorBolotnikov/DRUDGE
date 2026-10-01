package cmd

import (
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func TestPrinter(t *testing.T) {
	sampleTask := &task.Task{ID: "3f9a1c2e-0b1d-4c2e-9f3a-1c2e0b1d4c2e", Title: "Add retry to uploader"}

	cases := []struct {
		name       string
		env        map[string]string
		print      func(out *printer)
		wantStdout string
		wantStderr string
	}{
		{
			name: "lines inside a group are indented and the result sits at column 0",
			print: func(out *printer) {
				out.header("Setting up")
				out.done("Created a")
				out.skip("b already exists")
				out.result("Set up")
			},
			wantStdout: "Setting up\n  ✓ Created a\n  · b already exists\n✓ Set up\n",
		},
		{
			name: "lines with no open group sit at column 0",
			print: func(out *printer) {
				out.done("Created a")
				out.skip("b already exists")
				out.warn("c is broken")
			},
			wantStdout: "✓ Created a\n· b already exists\n",
			wantStderr: "! c is broken\n",
		},
		{
			name: "a second header gets one blank line before it",
			print: func(out *printer) {
				out.header("First")
				out.done("a")
				out.result("First done")
				out.header("Second")
				out.done("b")
				out.result("Second done")
			},
			wantStdout: "First\n  ✓ a\n✓ First done\n\nSecond\n  ✓ b\n✓ Second done\n",
		},
		{
			name: "a header right after a header gets one blank line before it",
			print: func(out *printer) {
				out.header("First")
				out.header("Second")
			},
			wantStdout: "First\n\nSecond\n",
		},
		{
			name: "a header after a line with no open group gets a blank line before it",
			print: func(out *printer) {
				out.done("a")
				out.header("Next")
			},
			wantStdout: "✓ a\n\nNext\n",
		},
		{
			name: "a warning counts as printed before a header",
			print: func(out *printer) {
				out.warn("a is broken")
				out.header("Next")
			},
			wantStdout: "\nNext\n",
			wantStderr: "! a is broken\n",
		},
		{
			name:       "a lone header has no blank line around it",
			print:      func(out *printer) { out.header("Only") },
			wantStdout: "Only\n",
		},
		{
			name: "a warning inside a group goes to stderr with the indent",
			print: func(out *printer) {
				out.header("Fetching")
				out.warn("Could not fetch %s", "main")
				out.result("Fetched")
			},
			wantStdout: "Fetching\n✓ Fetched\n",
			wantStderr: "  ! Could not fetch main\n",
		},
		{
			name: "a step ends with an ellipsis",
			print: func(out *printer) {
				out.step("Fetching %s", "main")
				out.header("Running")
				out.step("Starting the agent")
			},
			wantStdout: "› Fetching main…\n\nRunning\n  › Starting the agent…\n",
		},
		{
			name: "a detail sits one level deeper than the lines around it",
			print: func(out *printer) {
				out.detail("sbx: %s", "pulling")
				out.header("Running")
				out.step("Creating sandbox")
				out.detail("sbx: %s", "pulling")
			},
			wantStdout: "    sbx: pulling\n\nRunning\n  › Creating sandbox…\n      sbx: pulling\n",
		},
		{
			name:       "a detail loses the ANSI codes of its text",
			print:      func(out *printer) { out.detail("sbx: %s", "\x1b[2K\x1b[32mpulling\x1b[0m image") },
			wantStdout: "    sbx: pulling image\n",
		},
		{
			name: "the values of consecutive fields line up",
			print: func(out *printer) {
				out.header("Running")
				out.result("Started")
				out.field("Branch", "drudge/3f9a-add-retry")
				out.field("Run dir", ".drudge/runs/3f9a1c2e")
				out.flush()
			},
			wantStdout: "Running\n✓ Started\n    Branch   drudge/3f9a-add-retry\n    Run dir  .drudge/runs/3f9a1c2e\n",
		},
		{
			name: "the next line ends a run of fields",
			print: func(out *printer) {
				out.field("A", "1")
				out.field("Longer", "2")
				out.done("Next")
				out.field("Branch", "3")
				out.warn("Broken")
			},
			wantStdout: "    A       1\n    Longer  2\n✓ Next\n    Branch  3\n",
			wantStderr: "! Broken\n",
		},
		{
			name: "fields in an open group sit at the detail indent of the group",
			print: func(out *printer) {
				out.header("Running")
				out.field("Branch", "main")
				out.result("Started")
			},
			wantStdout: "Running\n      Branch  main\n✓ Started\n",
		},
		{
			name:       "a field with no value prints its label alone",
			print:      func(out *printer) { out.field("Commands", ""); out.flush() },
			wantStdout: "    Commands\n",
		},
		{
			name: "a block in a group sits at the field indent with a blank line around it",
			print: func(out *printer) {
				out.header("Dry run")
				out.field("Prompt from", "built-in prompt")
				out.block("Fix login\n\nSSO is broken\n")
				out.field("Commands", "")
				out.block(`"sbx" "ls"`)
				out.skipResult("Nothing ran")
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
			print:      func(out *printer) { out.block("a\nb") },
			wantStdout: "    a\n    b\n",
		},
		{
			name: "two blocks in a row get one blank line between them",
			print: func(out *printer) {
				out.block("a")
				out.block("b")
			},
			wantStdout: "    a\n\n    b\n",
		},
		{
			name: "a header after a block gets one blank line before it",
			print: func(out *printer) {
				out.block("a")
				out.header("Next")
			},
			wantStdout: "    a\n\nNext\n",
		},
		{
			name: "a warning after a block gets the blank line before it",
			print: func(out *printer) {
				out.block("a")
				out.warn("b is broken")
				out.done("c")
			},
			wantStdout: "    a\n\n✓ c\n",
			wantStderr: "! b is broken\n",
		},
		{
			name: "an empty block prints nothing",
			print: func(out *printer) {
				out.done("a")
				out.block("\n")
				out.done("b")
			},
			wantStdout: "✓ a\n✓ b\n",
		},
		{
			name:       "a task reads as its short id, two spaces and its title",
			print:      func(out *printer) { out.done("Picked %s", out.task(sampleTask)) },
			wantStdout: "✓ Picked 3f9a1c2e  Add retry to uploader\n",
		},
		{
			name:       "forced color makes the short id of a task bold",
			env:        map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print:      func(out *printer) { out.done("Picked %s", out.task(sampleTask)) },
			wantStdout: "\x1b[32m✓\x1b[0m Picked \x1b[1m3f9a1c2e\x1b[0m  Add retry to uploader\n",
		},
		{
			name: "forced color mutes the step glyph, the detail and the field label",
			env:  map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print: func(out *printer) {
				out.step("Creating sandbox")
				out.detail("sbx: \x1b[31mpulling\x1b[0m")
				out.field("Branch", "main")
				out.flush()
			},
			wantStdout: "\x1b[2m›\x1b[0m Creating sandbox…\n" +
				"    \x1b[2msbx: pulling\x1b[0m\n" +
				"    \x1b[2mBranch\x1b[0m  main\n",
		},
		{
			name: "forced color leaves a block in the default color",
			env:  map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print: func(out *printer) {
				out.field("Commands", "")
				out.block(`"sbx" "ls"`)
			},
			wantStdout: "    \x1b[2mCommands\x1b[0m\n\n" + `    "sbx" "ls"` + "\n",
		},
		{
			name:       "forced color mutes the glyph of a skipped result",
			env:        map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print:      func(out *printer) { out.skipResult("Nothing ran") },
			wantStdout: "\x1b[2m·\x1b[0m Nothing ran\n",
		},
		{
			name:       "forced color makes the header bold",
			env:        map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print:      func(out *printer) { out.header("Setting up") },
			wantStdout: "\x1b[1mSetting up\x1b[0m\n",
		},
		{
			name: "forced color paints the glyphs in their roles",
			env:  map[string]string{"NO_COLOR": "", "FORCE_COLOR": "1"},
			print: func(out *printer) {
				out.header("Setting up")
				out.done("Created a")
				out.skip("b already exists")
				out.warn("c is broken")
				out.result("Set up")
			},
			wantStdout: "\x1b[1mSetting up\x1b[0m\n" +
				"  \x1b[32m✓\x1b[0m Created a\n" +
				"  \x1b[2m·\x1b[0m b already exists\n" +
				"\x1b[32m✓\x1b[0m Set up\n",
			wantStderr: "  \x1b[33m!\x1b[0m c is broken\n",
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
			out := newPrinter(newThemedLogger(palette), palette)

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
