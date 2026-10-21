package common

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

const questionSchema = `{"properties": {"drudger": {"type": "object", "properties": {
	"environment": {"type": "string", "enum": ["docker-sbx"], "default": "docker-sbx"},
	"harness": {"type": "string", "enum": ["claude-code", "opencode"], "default": "claude-code"},
	"model": {"type": "string"},
	"wrongDefault": {"type": "string", "enum": ["a", "b"], "default": "c"}
}}}}`

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
