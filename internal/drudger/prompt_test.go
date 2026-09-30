package drudger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestRenderPrompt(t *testing.T) {
	cases := []struct {
		name            string
		template        string
		taskToRun       *task.Task
		workspace       promptWorkspace
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
			name:      "substitutes the workspace placeholders",
			template:  "{{taskTitle}} {{taskDescription}} on {{branch}} off {{defaultBranch}}",
			taskToRun: &task.Task{Title: "Fix login", Description: "SSO is broken"},
			workspace: promptWorkspace{Branch: "drudge/task-1-fix-login", DefaultBranch: "main"},
			want:      "Fix login SSO is broken on drudge/task-1-fix-login off main",
		},
		{
			name:      "workspace placeholders may be absent from the template",
			template:  "{{taskTitle}}: {{taskDescription}}",
			taskToRun: &task.Task{Title: "Fix login", Description: "SSO is broken"},
			workspace: promptWorkspace{Branch: "drudge/task-1-fix-login", DefaultBranch: "main"},
			want:      "Fix login: SSO is broken",
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
			got, err := renderPrompt(testCase.template, testCase.taskToRun, testCase.workspace)

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

	prompt, err := renderPrompt(defaultPromptTemplate, taskToRun, promptWorkspace{Branch: testTaskBranch, DefaultBranch: testDefaultBranch})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{taskToRun.Title, taskToRun.Description, taskToRun.TicketID, testTaskBranch} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected rendered default prompt to contain %q, got %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "{{") {
		t.Errorf("expected no placeholders left in the rendered default prompt, got %q", prompt)
	}
}

// setupPromptDirs chdirs into a temp working directory and points the home
// directory at another one, so both prompt directories resolve inside temp
// dirs owned by the test.
func setupPromptDirs(t *testing.T) string {
	t.Helper()
	workDir := t.TempDir()
	home := t.TempDir()

	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", home)

	t.Cleanup(func() {
		os.Chdir(origCwd)
		os.Setenv("HOME", origHome)
	})
	return home
}

func writePromptFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := common.EnsureDir(dir); err != nil {
		t.Fatalf("could not create prompts dir: %v", err)
	}
	if err := common.WriteFile(filepath.Join(dir, name), content); err != nil {
		t.Fatalf("could not write prompt file: %v", err)
	}
}

func TestResolvePromptTemplate(t *testing.T) {
	const (
		fileName = "impl.md"
		template = "custom {{taskTitle}} {{taskDescription}}"
	)

	cases := []struct {
		name            string
		shouldNamePath  bool
		shouldWriteFile bool
		want            string
		wantSource      string
		wantErrContains string
	}{
		{
			name:       "falls back to the default when no prompt file is named",
			want:       defaultPromptTemplate,
			wantSource: promptSourceDefault,
		},
		{
			name:            "reads the named prompt file",
			shouldNamePath:  true,
			shouldWriteFile: true,
			want:            template,
			wantSource:      fileName,
		},
		{
			name:            "missing prompt file is an error",
			shouldNamePath:  true,
			wantErrContains: fileName,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupPromptDirs(t)
			if testCase.shouldWriteFile {
				writePromptFile(t, common.LocalPromptsDir(), fileName, template)
			}
			path := ""
			if testCase.shouldNamePath {
				path = filepath.Join(common.LocalPromptsDir(), fileName)
			}

			got, source, err := resolvePromptTemplate(path)

			if testCase.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected an error naming %s, got template %q", testCase.wantErrContains, got)
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
				t.Errorf("expected template %q, got %q", testCase.want, got)
			}
			if !strings.HasSuffix(source, testCase.wantSource) {
				t.Errorf("expected source ending in %q, got %q", testCase.wantSource, source)
			}
		})
	}
}
