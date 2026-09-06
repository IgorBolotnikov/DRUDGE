package exec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandRunner_Run(t *testing.T) {
	cases := []struct {
		name            string
		argv            []string
		want            string
		wantErrContains string
	}{
		{
			name: "returns stdout",
			argv: []string{"echo", "hello"},
			want: "hello\n",
		},
		{
			name: "passes an argument with spaces and newlines through untouched",
			argv: []string{"printf", "%s", "first line\nsecond line"},
			want: "first line\nsecond line",
		},
		{
			name:            "reports what a failing command wrote to stderr",
			argv:            []string{"sh", "-c", "echo boom >&2; exit 1"},
			wantErrContains: "boom",
		},
		{
			name:            "unknown binary is an error",
			argv:            []string{"drudge-no-such-binary"},
			wantErrContains: "drudge-no-such-binary",
		},
		{
			name:            "empty argv is an error",
			argv:            nil,
			wantErrContains: "empty command",
		},
	}

	runner := NewCommandRunner()

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := runner.Run(testCase.argv)

			if testCase.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected an error naming %s, got output %q", testCase.wantErrContains, got)
				}
				if !strings.Contains(err.Error(), testCase.wantErrContains) {
					t.Errorf("expected error to name %s, got %q", testCase.wantErrContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.want {
				t.Errorf("expected output %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestCommandRunner_Start(t *testing.T) {
	cases := []struct {
		name            string
		argv            []string
		wantErrContains string
	}{
		{
			name: "starts a command",
			argv: []string{"true"},
		},
		{
			name:            "unknown binary is an error",
			argv:            []string{"drudge-no-such-binary"},
			wantErrContains: "drudge-no-such-binary",
		},
		{
			name:            "empty argv is an error",
			argv:            nil,
			wantErrContains: "empty command",
		},
	}

	runner := NewCommandRunner()

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := runner.Start(testCase.argv)

			if testCase.wantErrContains != "" {
				if err == nil {
					t.Fatal("expected an error, got none")
				}
				if !strings.Contains(err.Error(), testCase.wantErrContains) {
					t.Errorf("expected error to name %s, got %q", testCase.wantErrContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// An agent outlives the command that launches it, so Start has to hand back
// control while the command it started is still going.
func TestCommandRunner_Start_DoesNotWaitForTheCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	runner := NewCommandRunner()

	if err := runner.Start([]string{"sh", "-c", "sleep 1; touch " + marker}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err == nil {
		t.Fatal("Start waited for the command to finish")
	}

	// The command has to carry on once Start has returned, so the work it was
	// given still lands.
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the started command never finished its work")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
