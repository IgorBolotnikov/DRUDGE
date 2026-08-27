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
