package exec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// generousTimeout is long enough that no command of these tests reaches it.
const generousTimeout = 30 * time.Second

func TestCommandRunner_Run(t *testing.T) {
	cases := []struct {
		name            string
		argv            []string
		timeout         time.Duration
		want            string
		wantStderr      string
		wantErrContains string
	}{
		{
			name:    "returns stdout",
			argv:    []string{"echo", "hello"},
			timeout: generousTimeout,
			want:    "hello\n",
		},
		{
			name:    "passes an argument with spaces and newlines through untouched",
			argv:    []string{"printf", "%s", "first line\nsecond line"},
			timeout: generousTimeout,
			want:    "first line\nsecond line",
		},
		{
			name:       "hands back a notice a succeeding command wrote to stderr",
			argv:       []string{"sh", "-c", "echo 'Starting sandboxd daemon...' >&2; echo hello"},
			timeout:    generousTimeout,
			want:       "hello\n",
			wantStderr: "Starting sandboxd daemon...",
		},
		{
			name:            "reports what a failing command wrote to stderr",
			argv:            []string{"sh", "-c", "echo boom >&2; exit 1"},
			timeout:         generousTimeout,
			wantStderr:      "boom",
			wantErrContains: "boom",
		},
		{
			name:            "unknown binary is an error",
			argv:            []string{"drudge-no-such-binary"},
			timeout:         generousTimeout,
			wantErrContains: "drudge-no-such-binary",
		},
		{
			name:            "empty argv is an error",
			argv:            nil,
			timeout:         generousTimeout,
			wantErrContains: "empty command",
		},
		{
			name:            "a command without a timeout is an error",
			argv:            []string{"echo", "hello"},
			wantErrContains: "positive timeout",
		},
	}

	runner := NewCommandRunner()

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, gotStderr, err := runner.Run(testCase.argv, testCase.timeout)

			if gotStderr != testCase.wantStderr {
				t.Errorf("expected stderr %q, got %q", testCase.wantStderr, gotStderr)
			}

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

func TestCommandRunner_RunEchoed(t *testing.T) {
	cases := []struct {
		name      string
		script    string
		want      string
		wantLines []string
	}{
		{
			name:      "echoes stdout and stderr and still returns stdout",
			script:    "echo out; sleep 0.1; echo err >&2",
			want:      "out\n",
			wantLines: []string{"out", "err"},
		},
		{
			name:      "echoes a line redrawn with carriage returns as it last read",
			script:    `printf '10%%\r50%%\r100%%\n'`,
			want:      "10%\r50%\r100%\n",
			wantLines: []string{"100%"},
		},
		{
			name:      "drops blank lines",
			script:    "echo first; echo; echo '   '; echo second",
			want:      "first\n\n   \nsecond\n",
			wantLines: []string{"first", "second"},
		},
		{
			name:      "echoes a last line with no newline",
			script:    "printf 'no newline'",
			want:      "no newline",
			wantLines: []string{"no newline"},
		},
	}

	runner := NewCommandRunner()

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var gotLines []string
			got, _, err := runner.RunEchoed([]string{"sh", "-c", testCase.script}, generousTimeout, func(line string) {
				gotLines = append(gotLines, line)
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.want {
				t.Errorf("expected output %q, got %q", testCase.want, got)
			}
			if !slices.Equal(gotLines, testCase.wantLines) {
				t.Errorf("expected echoed lines %q, got %q", testCase.wantLines, gotLines)
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

func TestCommandRunner_Start_DoesNotWaitForTheCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	runner := NewCommandRunner()

	if err := runner.Start([]string{"sh", "-c", "sleep 1; touch " + marker}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err == nil {
		t.Fatal("Start waited for the command to finish")
	}

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

func TestCommandRunner_Run_KillsACommandThatOutrunsItsTimeout(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	argv := []string{"sh", "-c", "sleep 2; touch " + marker}
	timeout := 100 * time.Millisecond
	runner := NewCommandRunner()

	started := time.Now()
	_, _, err := runner.Run(argv, timeout)
	waited := time.Since(started)

	if err == nil {
		t.Fatal("expected an error, got none")
	}
	for _, want := range []string{"sh", timeout.String()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to name %s, got %q", want, err)
		}
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected a deadline error the caller can recognise, got %q", err)
	}

	if waited > time.Second {
		t.Errorf("Run waited %s for a command it was told to kill after %s", waited, timeout)
	}

	time.Sleep(2 * time.Second)
	if _, err := os.Stat(marker); err == nil {
		t.Error("the killed command went on working after Run returned")
	}
}

func TestCommandRunner_WithoutEnv(t *testing.T) {
	t.Setenv("DRUDGE_TEST_DROPPED", "inherited")
	t.Setenv("DRUDGE_TEST_KEPT", "inherited")
	argv := []string{"sh", "-c", "echo ${DRUDGE_TEST_DROPPED-unset} ${DRUDGE_TEST_KEPT-unset}"}

	cases := []struct {
		name   string
		runner *CommandRunner
		want   string
	}{
		{
			name:   "a runner that drops nothing",
			runner: NewCommandRunner(),
			want:   "inherited inherited\n",
		},
		{
			name:   "a runner that drops one variable",
			runner: NewCommandRunner().WithoutEnv("DRUDGE_TEST_DROPPED"),
			want:   "unset inherited\n",
		},
		{
			name:   "a runner that drops a variable nobody set",
			runner: NewCommandRunner().WithoutEnv("DRUDGE_TEST_NEVER_SET"),
			want:   "inherited inherited\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stdout, _, err := testCase.runner.Run(argv, generousTimeout)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stdout != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, stdout)
			}
		})
	}
}

func TestCommandRunner_WithoutEnv_LeavesTheOriginalRunnerAlone(t *testing.T) {
	t.Setenv("DRUDGE_TEST_DROPPED", "inherited")
	runner := NewCommandRunner()
	runner.WithoutEnv("DRUDGE_TEST_DROPPED")

	stdout, _, err := runner.Run([]string{"sh", "-c", "echo ${DRUDGE_TEST_DROPPED-unset}"}, generousTimeout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "inherited\n" {
		t.Errorf("expected the original runner to pass the variable on, got %q", stdout)
	}
}
