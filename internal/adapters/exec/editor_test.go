package exec

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEditor(t *testing.T) {
	cases := []struct {
		name     string
		editor   string
		wantName string
	}{
		{name: "$EDITOR names the editor", editor: "code --wait", wantName: "code --wait"},
		{name: "vi stands in for an unset $EDITOR", wantName: "vi"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(editorEnv, testCase.editor)

			if got := NewEditor().Name(); got != testCase.wantName {
				t.Errorf("expected editor %q, got %q", testCase.wantName, got)
			}
		})
	}
}

func TestEditor_Edit(t *testing.T) {
	cases := []struct {
		name     string
		editor   string
		fileName string
	}{
		{name: "an editor with arguments", editor: "printf %s", fileName: "draft.md"},
		{name: "a path with shell characters", editor: "printf %s", fileName: "it's $HOME; a draft.md"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(editorEnv, testCase.editor)
			path := filepath.Join(t.TempDir(), testCase.fileName)

			var output strings.Builder

			err := NewEditor().Edit(path, strings.NewReader(""), &output, &output)

			if err != nil {
				t.Fatalf("expected the editor to run, got %v", err)
			}
			if output.String() != path {
				t.Errorf("expected the editor to get %q, got %q", path, output.String())
			}
		})
	}
}
