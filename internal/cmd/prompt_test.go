package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
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

func concurrencyQuestion() common.Question {
	return common.Question{
		Key:     "drudger.maxConcurrentDrudgers",
		Label:   "Drudgers",
		Kind:    common.QuestionNumber,
		Default: "3",
		Validate: func(answer string) error {
			if answer == "0" {
				return errors.New("must be positive")
			}
			return nil
		},
	}
}

func modelQuestion() common.Question {
	return common.Question{Key: "drudger.model", Label: "Model", Kind: common.QuestionText, DefaultLabel: "harness default"}
}

func TestAskQuestions_Typed(t *testing.T) {
	cases := []struct {
		name           string
		question       common.Question
		input          string
		want           string
		wantLine       string
		wantRefusals   int
		wantEndOfInput bool
	}{
		{name: "enter takes the default", question: concurrencyQuestion(), input: "\n", want: "3", wantLine: "Drudgers: 3\n"},
		{name: "a typed value is returned", question: concurrencyQuestion(), input: "5\n", want: "5", wantLine: "Drudgers: 5\n"},
		{name: "spaces around the value are dropped", question: modelQuestion(), input: "  opus \n", want: "opus", wantLine: "Model: opus\n"},
		{name: "enter on an empty default shows the default label", question: modelQuestion(), input: "\n", want: "", wantLine: "Model: harness default\n"},
		{name: "a refused value asks again", question: concurrencyQuestion(), input: "0\n0\n4\n", want: "4", wantLine: "Drudgers: 4\n", wantRefusals: 2},
		{name: "input that ends asks for an answer", question: concurrencyQuestion(), input: "5", wantEndOfInput: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			var output bytes.Buffer

			answers, err := askQuestions(strings.NewReader(testCase.input), &output, theme.NewTheme(theme.DefaultTheme()), []common.Question{testCase.question})

			if testCase.wantEndOfInput {
				if err == nil || errors.Is(err, errCancelled) {
					t.Fatalf("err = %v, want an error about the input ending", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("askQuestions: %v", err)
			}
			if got := answers[testCase.question.Key]; got != testCase.want {
				t.Errorf("answer = %q, want %q", got, testCase.want)
			}
			if !strings.HasSuffix(output.String(), testCase.wantLine) {
				t.Errorf("output = %q, want it to end with %q", output.String(), testCase.wantLine)
			}
			if got := strings.Count(output.String(), "! must be positive\n"); got != testCase.wantRefusals {
				t.Errorf("output = %q, want %d refusals", output.String(), testCase.wantRefusals)
			}
		})
	}
}

func TestAskQuestions_TypedPrompt(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var output bytes.Buffer

	if _, err := askQuestions(strings.NewReader("\n"), &output, theme.NewTheme(theme.DefaultTheme()), []common.Question{modelQuestion()}); err != nil {
		t.Fatalf("askQuestions: %v", err)
	}

	if wantPrompt := "Model [harness default]: "; !strings.HasPrefix(output.String(), wantPrompt) {
		t.Errorf("output = %q, want it to start with %q", output.String(), wantPrompt)
	}
}

func TestAskQuestions_TypedCtrlCCancels(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	input, inputWriter := io.Pipe()
	defer inputWriter.Close()
	var output bytes.Buffer
	asker := newPrompter(input, &output, theme.NewTheme(theme.DefaultTheme()))
	asker.interrupts = func() (<-chan os.Signal, func()) {
		interrupts := make(chan os.Signal, 1)
		interrupts <- os.Interrupt
		return interrupts, func() {}
	}

	_, err := asker.ask([]common.Question{concurrencyQuestion()})

	if !errors.Is(err, errCancelled) {
		t.Fatalf("err = %v, want it to be cancelled", err)
	}
	if code := ExitCode(err); code != cancelledExitCode {
		t.Errorf("ExitCode = %d, want %d", code, cancelledExitCode)
	}
	if strings.Contains(output.String(), "Drudgers:") {
		t.Errorf("output = %q, want no answer line", output.String())
	}
}

func TestAskQuestions_SkipsAQuestionWhoseConditionFails(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	conditional := modelQuestion()
	conditional.Condition = &common.QuestionCondition{Key: "drudger.maxConcurrentDrudgers", Answer: "5"}
	var output bytes.Buffer

	answers, err := askQuestions(strings.NewReader("4\nopus\n"), &output, theme.NewTheme(theme.DefaultTheme()), []common.Question{concurrencyQuestion(), conditional})
	if err != nil {
		t.Fatalf("askQuestions: %v", err)
	}

	if _, ok := answers[conditional.Key]; ok {
		t.Errorf("answers = %v, want no answer for %s", answers, conditional.Key)
	}
}

func TestExitCode(t *testing.T) {
	if code := ExitCode(errors.New("boom")); code != failedExitCode {
		t.Errorf("ExitCode = %d, want %d", code, failedExitCode)
	}
}
