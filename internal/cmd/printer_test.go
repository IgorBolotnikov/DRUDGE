package cmd

import (
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func TestPrinter(t *testing.T) {
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
