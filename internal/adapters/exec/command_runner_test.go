package exec

import (
	"strings"
	"testing"
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
