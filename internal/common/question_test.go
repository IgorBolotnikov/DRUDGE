package common

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

const questionSchema = `{"properties": {"drudger": {"type": "object", "properties": {
	"environment": {"type": "string", "enum": ["docker-sbx"], "default": "docker-sbx"},
	"harness": {"type": "string", "enum": ["claude-code", "opencode"], "default": "claude-code"},
	"model": {"type": "string"},
	"wrongDefault": {"type": "string", "enum": ["a", "b"], "default": "c"},
	"maxConcurrent": {"type": "integer", "default": 3},
	"wrongNumberDefault": {"type": "integer", "default": 0},
	"isOn": {"type": "boolean", "default": true},
	"isOff": {"type": "boolean"}
}}}}`

func positive(value int) error {
	if value < 1 {
		return errors.New("must be positive")
	}
	return nil
}

func TestNewBoolQuestion(t *testing.T) {
	cases := []struct {
		name         string
		key          string
		wantDefault  string
		wantErrInMsg string
	}{
		{name: "a default of true", key: "drudger.isOn", wantDefault: AnswerYes},
		{name: "no default takes no", key: "drudger.isOff", wantDefault: AnswerNo},
		{name: "a key that is not a boolean", key: "drudger.model", wantErrInMsg: "want a boolean"},
		{name: "a missing key", key: "drudger.missing", wantErrInMsg: "has no key"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			question, err := NewBoolQuestion([]byte(questionSchema), testCase.key, "Label")
			if testCase.wantErrInMsg != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrInMsg) {
					t.Fatalf("err = %v, want one containing %q", err, testCase.wantErrInMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewBoolQuestion: %v", err)
			}
			wantOptions := []QuestionOption{{Value: AnswerNo, Label: AnswerNo}, {Value: AnswerYes, Label: AnswerYes}}
			if !slices.Equal(question.Options, wantOptions) {
				t.Errorf("Options = %v, want %v", question.Options, wantOptions)
			}
			if question.Default != testCase.wantDefault {
				t.Errorf("Default = %q, want %q", question.Default, testCase.wantDefault)
			}
		})
	}
}

func TestNewNumberQuestion(t *testing.T) {
	question, err := NewNumberQuestion([]byte(questionSchema), "drudger.maxConcurrent", "Label", positive)
	if err != nil {
		t.Fatalf("NewNumberQuestion: %v", err)
	}
	if question.Kind != QuestionNumber || question.Default != "3" {
		t.Errorf("Kind = %q, Default = %q, want %q and %q", question.Kind, question.Default, QuestionNumber, "3")
	}

	cases := []struct {
		answer       string
		wantErrInMsg string
	}{
		{answer: "5"},
		{answer: "0", wantErrInMsg: "must be positive"},
		{answer: "five", wantErrInMsg: "is not a whole number"},
		{answer: "", wantErrInMsg: "is not a whole number"},
	}
	for _, testCase := range cases {
		t.Run(testCase.answer, func(t *testing.T) {
			err := question.Validate(testCase.answer)
			if testCase.wantErrInMsg == "" {
				if err != nil {
					t.Errorf("Validate(%q): %v", testCase.answer, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.wantErrInMsg) {
				t.Errorf("Validate(%q) = %v, want an error containing %q", testCase.answer, err, testCase.wantErrInMsg)
			}
		})
	}
}

func TestNewNumberQuestion_Refuses(t *testing.T) {
	cases := []struct {
		name         string
		key          string
		wantErrInMsg string
	}{
		{name: "a default validate refuses", key: "drudger.wrongNumberDefault", wantErrInMsg: "must be positive"},
		{name: "a key that is not an integer", key: "drudger.model", wantErrInMsg: "want an integer"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewNumberQuestion([]byte(questionSchema), testCase.key, "Label", positive)
			if err == nil || !strings.Contains(err.Error(), testCase.wantErrInMsg) {
				t.Errorf("err = %v, want one containing %q", err, testCase.wantErrInMsg)
			}
		})
	}
}

func TestNewTextQuestion(t *testing.T) {
	cases := []struct {
		name         string
		key          string
		wantErrInMsg string
	}{
		{name: "a string key", key: "drudger.model"},
		{name: "a key that is not a string", key: "drudger.maxConcurrent", wantErrInMsg: "want a string"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			question, err := NewTextQuestion([]byte(questionSchema), testCase.key, "Label", "shown default")
			if testCase.wantErrInMsg != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrInMsg) {
					t.Fatalf("err = %v, want one containing %q", err, testCase.wantErrInMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTextQuestion: %v", err)
			}
			if question.Kind != QuestionText || question.Default != "" || question.DefaultLabel != "shown default" {
				t.Errorf("question = %+v, want an empty text question shown as %q", question, "shown default")
			}
		})
	}
}

func TestNewSelectQuestion(t *testing.T) {
	cases := []struct {
		name         string
		key          string
		wantOptions  []string
		wantDefault  string
		wantIsAsked  bool
		wantErrInMsg string
	}{
		{name: "an enum of two", key: "drudger.harness", wantOptions: []string{"claude-code", "opencode"}, wantDefault: "claude-code", wantIsAsked: true},
		{name: "an enum of one is not asked", key: "drudger.environment", wantOptions: []string{"docker-sbx"}, wantDefault: "docker-sbx", wantIsAsked: false},
		{name: "a missing key", key: "drudger.missing", wantErrInMsg: "has no key"},
		{name: "a key without an enum", key: "drudger.model", wantErrInMsg: "has no enum"},
		{name: "a default outside the enum", key: "drudger.wrongDefault", wantErrInMsg: "is not one of"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			question, err := NewSelectQuestion([]byte(questionSchema), testCase.key, "Label")
			if testCase.wantErrInMsg != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrInMsg) {
					t.Fatalf("err = %v, want one containing %q", err, testCase.wantErrInMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewSelectQuestion: %v", err)
			}
			var gotOptions []string
			for _, option := range question.Options {
				gotOptions = append(gotOptions, option.Value)
			}
			if !slices.Equal(gotOptions, testCase.wantOptions) {
				t.Errorf("Options = %v, want %v", gotOptions, testCase.wantOptions)
			}
			if question.Default != testCase.wantDefault {
				t.Errorf("Default = %q, want %q", question.Default, testCase.wantDefault)
			}
			if question.IsAsked() != testCase.wantIsAsked {
				t.Errorf("IsAsked = %v, want %v", question.IsAsked(), testCase.wantIsAsked)
			}
		})
	}
}

func TestResolveAnswers(t *testing.T) {
	var questions []Question
	for _, key := range []string{"drudger.environment", "drudger.harness"} {
		question, err := NewSelectQuestion([]byte(questionSchema), key, "Label")
		if err != nil {
			t.Fatalf("NewSelectQuestion: %v", err)
		}
		questions = append(questions, question)
	}

	cases := []struct {
		name    string
		answers Answers
		want    Answers
		wantErr bool
	}{
		{name: "no answers take the defaults", answers: nil, want: Answers{"drudger.environment": "docker-sbx", "drudger.harness": "claude-code"}},
		{name: "an answer is kept", answers: Answers{"drudger.harness": "opencode"}, want: Answers{"drudger.environment": "docker-sbx", "drudger.harness": "opencode"}},
		{name: "a question that is not asked takes its only option", answers: Answers{"drudger.environment": "other"}, want: Answers{"drudger.environment": "docker-sbx", "drudger.harness": "claude-code"}},
		{name: "an answer outside the options", answers: Answers{"drudger.harness": "vim"}, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveAnswers(questions, testCase.answers)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("ResolveAnswers = %v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveAnswers: %v", err)
			}
			if !maps.Equal(got, testCase.want) {
				t.Errorf("ResolveAnswers = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestResolveAnswers_Condition(t *testing.T) {
	isOn, err := NewBoolQuestion([]byte(questionSchema), "drudger.isOff", "Label")
	if err != nil {
		t.Fatalf("NewBoolQuestion: %v", err)
	}
	harness, err := NewSelectQuestion([]byte(questionSchema), "drudger.harness", "Label")
	if err != nil {
		t.Fatalf("NewSelectQuestion: %v", err)
	}
	harness.Condition = &QuestionCondition{Key: isOn.Key, Answer: AnswerYes}
	questions := []Question{isOn, harness}

	cases := []struct {
		name    string
		answers Answers
		want    Answers
	}{
		{name: "a condition that holds resolves the question", answers: Answers{"drudger.isOff": AnswerYes}, want: Answers{"drudger.isOff": AnswerYes, "drudger.harness": "claude-code"}},
		{name: "a condition that fails leaves the question out", answers: Answers{"drudger.harness": "opencode"}, want: Answers{"drudger.isOff": AnswerNo}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveAnswers(questions, testCase.answers)
			if err != nil {
				t.Fatalf("ResolveAnswers: %v", err)
			}
			if !maps.Equal(got, testCase.want) {
				t.Errorf("ResolveAnswers = %v, want %v", got, testCase.want)
			}
		})
	}
}
