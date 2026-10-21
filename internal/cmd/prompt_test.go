package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func harnessQuestion() common.Question {
	return common.Question{
		Key:     "drudger.harness",
		Label:   "Harness",
		Kind:    common.QuestionSelect,
		Options: []common.QuestionOption{{Value: "claude-code", Label: "claude-code"}, {Value: "opencode", Label: "opencode"}, {Value: "aider", Label: "aider"}},
		Default: "opencode",
	}
}

func TestAskQuestions(t *testing.T) {
	cases := []struct {
		name           string
		keys           string
		want           string
		wantCancelled  bool
		wantEndOfInput bool
	}{
		{name: "enter picks the default", keys: "\r", want: "opencode"},
		{name: "j moves down", keys: "j\r", want: "aider"},
		{name: "k moves up", keys: "k\r", want: "claude-code"},
		{name: "arrow down moves down", keys: "\x1b[B\r", want: "aider"},
		{name: "arrow up moves up", keys: "\x1b[A\r", want: "claude-code"},
		{name: "an application mode arrow moves", keys: "\x1bOA\r", want: "claude-code"},
		{name: "movement stops at the top", keys: "kkk\r", want: "claude-code"},
		{name: "movement stops at the bottom", keys: "\x1b[B\x1b[B\x1b[Bj\r", want: "aider"},
		{name: "other keys are ignored", keys: "q\x1b[Cx\n", want: "opencode"},
		{name: "ctrl-c cancels", keys: "\x03", wantCancelled: true},
		{name: "esc cancels", keys: "\x1b", wantCancelled: true},
		{name: "esc before another key cancels", keys: "\x1bj\r", wantCancelled: true},
		{name: "input that ends asks to pick", keys: "j", wantEndOfInput: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			question := harnessQuestion()
			var output bytes.Buffer

			answers, err := askQuestions(strings.NewReader(testCase.keys), &output, theme.NewTheme(theme.DefaultTheme()), []common.Question{question})

			switch {
			case testCase.wantCancelled:
				if !errors.Is(err, errCancelled) {
					t.Fatalf("err = %v, want it to be cancelled", err)
				}
				if code := ExitCode(err); code != cancelledExitCode {
					t.Errorf("ExitCode = %d, want %d", code, cancelledExitCode)
				}
				if strings.Contains(output.String(), question.Label+":") {
					t.Errorf("output = %q, want no picked line", output.String())
				}
				return
			case testCase.wantEndOfInput:
				if err == nil || errors.Is(err, errCancelled) {
					t.Fatalf("err = %v, want an error about the input ending", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("askQuestions: %v", err)
			}
			if got := answers[question.Key]; got != testCase.want {
				t.Errorf("answer = %q, want %q", got, testCase.want)
			}
			if wantLine := "Harness: " + testCase.want + "\r\n"; !strings.HasSuffix(output.String(), wantLine) {
				t.Errorf("output = %q, want it to end with %q", output.String(), wantLine)
			}
		})
	}
}

func TestAskQuestions_DrawsTheList(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var output bytes.Buffer

	if _, err := askQuestions(strings.NewReader("\r"), &output, theme.NewTheme(theme.DefaultTheme()), []common.Question{harnessQuestion()}); err != nil {
		t.Fatalf("askQuestions: %v", err)
	}

	wantList := "Harness\r\n" +
		"  claude-code\r\n" +
		"› opencode (default)\r\n" +
		"  aider\r\n"
	if !strings.HasPrefix(output.String(), wantList) {
		t.Errorf("output = %q, want it to start with %q", output.String(), wantList)
	}
}

func TestExitCode(t *testing.T) {
	if code := ExitCode(errors.New("boom")); code != failedExitCode {
		t.Errorf("ExitCode = %d, want %d", code, failedExitCode)
	}
}
