package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugFrom(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Project", "my-project"},
		{"hello world", "hello-world"},
		{"  spaces  ", "--spaces--"},
		{"Single", "single"},
		{"", ""},
		{"MIXED CASE", "mixed-case"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SlugFrom(tt.input)
			if result != tt.expected {
				t.Errorf("SlugFrom(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatFrontMatter(t *testing.T) {
	metadata := map[string]string{
		"name":    "test",
		"created": "2025-01-01",
		"slug":    "abc",
	}
	result := FormatFrontMatter(metadata)

	if !strings.HasPrefix(result, "---\n") {
		t.Errorf("expected --- prefix, got %q", result)
	}
	if !strings.Contains(result, "---\n") {
		t.Error("missing closing ---")
	}

	if !strings.Contains(result, "created: 2025-01-01") {
		t.Error("missing created key")
	}
	if !strings.Contains(result, "name: test") {
		t.Error("missing name key")
	}
	if !strings.Contains(result, "slug: abc") {
		t.Error("missing slug key")
	}
}

func TestFormatFrontMatter_Sorted(t *testing.T) {
	metadata := map[string]string{
		"zzz":   "last",
		"alpha": "first",
		"beta":  "second",
	}
	result := FormatFrontMatter(metadata)

	lines := strings.Split(result, "\n")
	var metaLines []string
	inMeta := false
	for _, line := range lines {
		if line == "---" {
			inMeta = !inMeta
			continue
		}
		if inMeta && line != "" {
			metaLines = append(metaLines, line)
		}
	}

	if metaLines[0] != "alpha: first" || metaLines[1] != "beta: second" || metaLines[2] != "zzz: last" {
		t.Errorf("metadata not sorted, got %v", metaLines)
	}
}

func TestFormatFrontMatter_Empty(t *testing.T) {
	result := FormatFrontMatter(nil)
	if result != "---\n---\n" {
		t.Errorf("expected empty front-matter, got %q", result)
	}
}

func TestWriteFileWithFrontMatter(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.md"

	metadata := map[string]string{
		"title": "Hello",
		"slug":  "1",
	}
	body := "some content\n"

	err := WriteFileWithFrontMatter(path, metadata, body)
	if err != nil {
		t.Fatalf("WriteFileWithFrontMatter: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	contents := string(data)
	if !strings.Contains(contents, "---\n") {
		t.Error("missing delimiter")
	}
	if !strings.Contains(contents, "title: Hello") {
		t.Error("missing metadata")
	}
	if !strings.Contains(contents, "some content") {
		t.Error("missing body content")
	}
}

func TestDrudgeDir(t *testing.T) {
	tests := []struct {
		home     string
		expected string
	}{
		{"/home/alice", "/home/alice/.drudge"},
		{"", ".drudge"},
		{"/root", "/root/.drudge"},
		{"/home/bob/", "/home/bob/.drudge"},
	}

	for _, tt := range tests {
		t.Run(tt.home, func(t *testing.T) {
			result := DrudgeDir(tt.home)
			if result != tt.expected {
				t.Errorf("DrudgeDir(%q) = %q, want %q", tt.home, result, tt.expected)
			}
		})
	}
}

func TestProjectsDir(t *testing.T) {
	tests := []struct {
		home     string
		expected string
	}{
		{"/home/alice", "/home/alice/.drudge/projects"},
		{"", ".drudge/projects"},
		{"/root", "/root/.drudge/projects"},
		{"/home/bob/", "/home/bob/.drudge/projects"},
	}

	for _, tt := range tests {
		t.Run(tt.home, func(t *testing.T) {
			result := ProjectsDir(tt.home)
			if result != tt.expected {
				t.Errorf("ProjectsDir(%q) = %q, want %q", tt.home, result, tt.expected)
			}
		})
	}
}

func TestProjectsDir_DerivedFromDrudgeDir(t *testing.T) {
	home := "/tmp/testhome"
	want := filepath.Join(DrudgeDir(home), "projects")
	got := ProjectsDir(home)
	if got != want {
		t.Errorf("ProjectsDir should equal DrudgeDir(home) + /projects, got %q, want %q", got, want)
	}
}

func TestParseFrontMatter_Basic(t *testing.T) {
	data := "---\nid: abc123\ntitle: My Task\nstatus: todo\n---\nSome content here\n"
	metadata, content := ParseFrontMatter(data)

	if metadata["id"] != "abc123" {
		t.Errorf("expected id=abc123, got %q", metadata["id"])
	}
	if metadata["title"] != "My Task" {
		t.Errorf("expected title=My Task, got %q", metadata["title"])
	}
	if metadata["status"] != "todo" {
		t.Errorf("expected status=todo, got %q", metadata["status"])
	}
	if content != "Some content here" {
		t.Errorf("expected content 'Some content here', got %q", content)
	}
}

func TestParseFrontMatter_Empty(t *testing.T) {
	metadata, content := ParseFrontMatter("---\n---\n")
	if len(metadata) != 0 {
		t.Errorf("expected empty metadata, got %v", metadata)
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestParseFrontMatter_NoDelimiters(t *testing.T) {
	metadata, content := ParseFrontMatter("just plain text\n")
	if len(metadata) != 0 {
		t.Errorf("expected empty metadata, got %v", metadata)
	}
	if content != "just plain text\n" {
		t.Errorf("expected content unchanged, got %q", content)
	}
}

func TestParseFrontMatter_RoundTrip(t *testing.T) {
	original := map[string]string{
		"id":           "uuid-123",
		"title":        "Fix login bug",
		"status":       "in-progress",
		"project_slug": "my-project",
		"created_at":   "2025-01-15T10:30:00Z",
		"updated_at":   "2025-01-16T14:00:00Z",
	}
	body := "Description goes here\nWith multiple lines\n"

	formatted := FormatFrontMatter(original)
	roundtrip, content := ParseFrontMatter(formatted + body)

	for k, v := range original {
		if roundtrip[k] != v {
			t.Errorf("roundtrip %s: expected %q, got %q", k, v, roundtrip[k])
		}
	}
	if content != "Description goes here\nWith multiple lines" {
		t.Errorf("content mismatch: expected 'Description goes here\\nWith multiple lines', got %q", content)
	}
}

func TestParseFrontMatter_MultilineContent(t *testing.T) {
	data := "---\ntitle: Task\n---\nLine 1\nLine 2\nLine 3\n"
	_, content := ParseFrontMatter(data)
	expected := "Line 1\nLine 2\nLine 3"
	if content != expected {
		t.Errorf("expected multiline content, got %q", content)
	}
}

func TestWriteFileAndReadFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "plain text", content: "hello"},
		{name: "several lines", content: "first\nsecond\n"},
		{name: "quotes and dollars survive", content: `a "quoted" $value and an 'apostrophe'`},
		{name: "empty content", content: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file.txt")

			if err := WriteFile(path, tt.content); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			got, err := ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if got != tt.content {
				t.Errorf("read back %q, want %q", got, tt.content)
			}
		})
	}
}

func TestWriteFile_Overwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")

	if err := WriteFile(path, "first"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := WriteFile(path, "second"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != "second" {
		t.Errorf("expected the second write to win, got %q", got)
	}
}

func TestReadFile_MissingFileNamesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.txt")

	_, err := ReadFile(path)
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("expected the error to name %s, got %q", path, err)
	}
}

func TestWorkDir(t *testing.T) {
	dir := t.TempDir()

	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origCwd) })

	got, err := WorkDir()
	if err != nil {
		t.Fatalf("WorkDir: %v", err)
	}

	// A temp dir can sit behind a symlink, so compare what the OS resolves.
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if got != want {
		t.Errorf("expected working directory %q, got %q", want, got)
	}
}

func TestRunDirPaths(t *testing.T) {
	const (
		workspace = "/home/dev/drudge"
		taskID    = "task-1"
	)
	runDir := RunDir(workspace, taskID)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "run directory", got: runDir, want: workspace + "/.drudge/runs/task-1"},
		{name: "prompt", got: RunPromptPath(runDir), want: workspace + "/.drudge/runs/task-1/prompt.txt"},
		{name: "stream", got: RunStreamPath(runDir), want: workspace + "/.drudge/runs/task-1/stream.jsonl"},
		{name: "stderr", got: RunStderrPath(runDir), want: workspace + "/.drudge/runs/task-1/stderr.log"},
		{name: "exit", got: RunExitPath(runDir), want: workspace + "/.drudge/runs/task-1/exit"},
		{name: "runs directory is local", got: LocalRunsDir(), want: ".drudge/runs"},
		{name: "local run directory", got: LocalRunDir(taskID), want: ".drudge/runs/task-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, tt.got)
			}
		})
	}
}
