package style

import (
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func TestStepSegments(t *testing.T) {
	cases := []struct {
		name string
		step drudger.Step
		want []Segment
	}{
		{
			name: "init",
			step: drudger.Step{Kind: drudger.StepInit, Summary: "session 02005590… claude-opus"},
			want: []Segment{{Text: "session 02005590… claude-opus", Role: theme.RoleMuted}},
		},
		{
			name: "agent text keeps its line breaks",
			step: drudger.Step{Kind: drudger.StepAgentText, Summary: "Found it.\nThe decoder is in streams.", Depth: 1},
			want: []Segment{{Text: "Found it.\nThe decoder is in streams."}},
		},
		{
			name: "tool call",
			step: drudger.Step{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go test ./..."},
			want: []Segment{
				{Text: "Bash   ", Role: theme.RolePrimary},
				{Text: " "},
				{Text: "go test ./...", Role: theme.RoleSecondary},
			},
		},
		{
			name: "tool call with a path summary",
			step: drudger.Step{Kind: drudger.StepToolCall, Tool: "Read", Summary: "internal/task/model.go"},
			want: []Segment{
				{Text: "Read   ", Role: theme.RolePrimary},
				{Text: " "},
				{Text: "internal/task/model.go", Role: theme.RolePath},
			},
		},
		{
			name: "tool call with a name wider than the tool column",
			step: drudger.Step{Kind: drudger.StepToolCall, Tool: "WebSearch", Summary: "go flock"},
			want: []Segment{
				{Text: "WebSearch", Role: theme.RolePrimary},
				{Text: " "},
				{Text: "go flock", Role: theme.RoleSecondary},
			},
		},
		{
			name: "tool call with no summary",
			step: drudger.Step{Kind: drudger.StepToolCall, Tool: "WebSearch"},
			want: []Segment{{Text: "WebSearch", Role: theme.RolePrimary}},
		},
		{
			name: "tool error",
			step: drudger.Step{Kind: drudger.StepToolError, Summary: "exit status 1"},
			want: []Segment{
				{Text: "  "},
				{Text: "✗ exit status 1", Role: theme.RoleError},
			},
		},
		{
			name: "final result that got shit done",
			step: drudger.Step{Kind: drudger.StepFinalResult, Summary: "got shit done · 42 turns"},
			want: []Segment{{Text: "got shit done · 42 turns", Role: theme.RoleSuccess}},
		},
		{
			name: "final result that fucked up",
			step: drudger.Step{Kind: drudger.StepFinalResult, Summary: "fucked up · 5 turns"},
			want: []Segment{{Text: "fucked up · 5 turns", Role: theme.RoleError}},
		},
		{
			name: "unrecognized line shows its raw text",
			step: drudger.Step{Kind: drudger.StepUnrecognized, Summary: "ignored", Raw: `{"type":"tool_progress"}`},
			want: []Segment{{Text: `{"type":"tool_progress"}`, Role: theme.RoleMuted}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := StepSegments(testCase.step); !slices.Equal(got, testCase.want) {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestStepIndent(t *testing.T) {
	cases := []struct {
		name  string
		depth int
		want  string
	}{
		{name: "the session of the Drudger", depth: 0, want: ""},
		{name: "a subagent", depth: 1, want: "  "},
		{name: "a subagent of a subagent", depth: 2, want: "    "},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := StepIndent(testCase.depth); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestStepHangingIndent(t *testing.T) {
	cases := []struct {
		name string
		step drudger.Step
		want string
	}{
		{
			name: "tool call hangs under its summary",
			step: drudger.Step{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go test ./..."},
			want: "        ",
		},
		{
			name: "tool call at depth",
			step: drudger.Step{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go test ./...", Depth: 1},
			want: "          ",
		},
		{
			name: "tool call with a name wider than the tool column",
			step: drudger.Step{Kind: drudger.StepToolCall, Tool: "WebSearch", Summary: "go flock"},
			want: "          ",
		},
		{
			name: "tool error hangs under its message",
			step: drudger.Step{Kind: drudger.StepToolError, Summary: "exit status 1"},
			want: "    ",
		},
		{
			name: "agent text at depth",
			step: drudger.Step{Kind: drudger.StepAgentText, Summary: "Found it.", Depth: 2},
			want: "    ",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := StepHangingIndent(testCase.step); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
