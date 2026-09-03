package runner

import (
	"strings"
	"testing"

	"drudge/internal/task"
)

func TestRenderPrompt(t *testing.T) {
	cases := []struct {
		name            string
		template        string
		taskToRun       *task.Task
		want            string
		wantErrContains string
	}{
		{
			name:      "substitutes every placeholder",
			template:  "title: {{taskTitle}}\ndesc: {{taskDescription}}\nticket: {{ticketID}}",
			taskToRun: &task.Task{Title: "Fix login", Description: "SSO is broken", TicketID: "PROJ-123"},
			want:      "title: Fix login\ndesc: SSO is broken\nticket: PROJ-123",
		},
		{
			name:      "leaves the ticket ID blank when the task has none",
			template:  "title: {{taskTitle}}\ndesc: {{taskDescription}}\nticket: {{ticketID}}",
			taskToRun: &task.Task{Title: "Fix login", Description: "SSO is broken"},
			want:      "title: Fix login\ndesc: SSO is broken\nticket: ",
		},
		{
			name:      "ticket placeholder may be absent from the template",
			template:  "{{taskTitle}}: {{taskDescription}}",
			taskToRun: &task.Task{Title: "Fix login", Description: "SSO is broken", TicketID: "PROJ-123"},
			want:      "Fix login: SSO is broken",
		},
		{
			name:      "substitutes a placeholder used more than once",
			template:  "{{taskTitle}} / {{taskTitle}} / {{taskDescription}}",
			taskToRun: &task.Task{Title: "Fix login", Description: "SSO is broken"},
			want:      "Fix login / Fix login / SSO is broken",
		},
		{
			name:      "keeps a multi line description as is",
			template:  "{{taskTitle}}\n{{taskDescription}}",
			taskToRun: &task.Task{Title: "Fix login", Description: "first line\n\nsecond line"},
			want:      "Fix login\nfirst line\n\nsecond line",
		},
		{
			name:            "missing title placeholder is an error",
			template:        "desc: {{taskDescription}}",
			taskToRun:       &task.Task{Title: "Fix login", Description: "SSO is broken"},
			wantErrContains: placeholderTaskTitle,
		},
		{
			name:            "missing description placeholder is an error",
			template:        "title: {{taskTitle}}",
			taskToRun:       &task.Task{Title: "Fix login", Description: "SSO is broken"},
			wantErrContains: placeholderTaskDescription,
		},
		{
			name:            "empty template is an error",
			template:        "",
			taskToRun:       &task.Task{Title: "Fix login", Description: "SSO is broken"},
			wantErrContains: placeholderTaskTitle,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := renderPrompt(testCase.template, testCase.taskToRun)

			if testCase.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected an error naming %s, got prompt %q", testCase.wantErrContains, got)
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
				t.Errorf("expected prompt %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestDefaultPromptTemplate_HasRequiredPlaceholders(t *testing.T) {
	for _, placeholder := range requiredPlaceholders {
		if !strings.Contains(defaultPromptTemplate, placeholder) {
			t.Errorf("default prompt template is missing the required %s placeholder", placeholder)
		}
	}
}

func TestDefaultPromptTemplate_RendersTaskDetails(t *testing.T) {
	taskToRun := &task.Task{Title: "Fix login", Description: "SSO is broken", TicketID: "PROJ-123"}

	prompt, err := renderPrompt(defaultPromptTemplate, taskToRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{taskToRun.Title, taskToRun.Description, taskToRun.TicketID} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected rendered default prompt to contain %q, got %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "{{") {
		t.Errorf("expected no placeholders left in the rendered default prompt, got %q", prompt)
	}
}
