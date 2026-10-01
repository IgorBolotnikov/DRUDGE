package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanup(t *testing.T) {
	cases := []struct {
		name            string
		isInstalled     bool
		isForced        bool
		answer          string
		wantStdout      func(drudgeDir string) string
		wantIsInstalled bool
	}{
		{
			name:        "a confirmed prompt removes the drudge dir",
			isInstalled: true,
			answer:      "y\n",
			wantStdout: func(drudgeDir string) string {
				return "Are you sure? [y/N]: ✓ Removed " + drudgeDir + "\n"
			},
		},
		{
			name:        "a declined prompt leaves the drudge dir",
			isInstalled: true,
			answer:      "n\n",
			wantStdout: func(drudgeDir string) string {
				return "Are you sure? [y/N]: · Left " + drudgeDir + " alone\n"
			},
			wantIsInstalled: true,
		},
		{
			name:        "a forced cleanup removes the drudge dir",
			isInstalled: true,
			isForced:    true,
			wantStdout: func(drudgeDir string) string {
				return "✓ Removed " + drudgeDir + "\n"
			},
		},
		{
			name: "a missing drudge dir has nothing to clean up",
			wantStdout: func(drudgeDir string) string {
				return "· Nothing to clean up, " + drudgeDir + " does not exist\n"
			},
		},
		{
			name:     "a forced cleanup of a missing drudge dir has nothing to clean up",
			isForced: true,
			wantStdout: func(drudgeDir string) string {
				return "· Nothing to clean up, " + drudgeDir + " does not exist\n"
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("NO_COLOR", "1")
			drudgeDir := filepath.Join(home, ".drudge")
			if testCase.isInstalled {
				if err := os.Mkdir(drudgeDir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			answerPath := filepath.Join(t.TempDir(), "answer")
			if err := os.WriteFile(answerPath, []byte(testCase.answer), 0o644); err != nil {
				t.Fatal(err)
			}
			answer, err := os.Open(answerPath)
			if err != nil {
				t.Fatal(err)
			}
			defer answer.Close()
			originalStdin := os.Stdin
			os.Stdin = answer
			defer func() { os.Stdin = originalStdin }()

			var stdout string
			captureStderr(func() {
				stdout = captureOutput(func() { err = cleanup(testCase.isForced) })
			})

			if err != nil {
				t.Fatalf("cleanup: %v", err)
			}
			if want := testCase.wantStdout(drudgeDir); stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
			if _, statErr := os.Stat(drudgeDir); (statErr == nil) != testCase.wantIsInstalled {
				t.Errorf("drudge dir exists = %v, want %v", statErr == nil, testCase.wantIsInstalled)
			}
		})
	}
}
