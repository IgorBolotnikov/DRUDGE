package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const draftTestProject = "test-project"

func TestFileDraftRepository_DraftPath(t *testing.T) {
	home := t.TempDir()
	repo := NewFileDraftRepository(home)

	path, err := repo.DraftPath(draftTestProject, task.NewTaskDraftName)
	if err != nil {
		t.Fatalf("DraftPath: %v", err)
	}

	want := filepath.Join(home, ".drudge", "projects", draftTestProject, "drafts", "new-task.md")
	if path != want {
		t.Errorf("expected path %q, got %q", want, path)
	}
}

func TestFileDraftRepository_ReadDraft(t *testing.T) {
	cases := []struct {
		name        string
		writes      []string
		isRemoved   bool
		wantContent string
		wantIsFound bool
	}{
		{name: "no draft is not found"},
		{name: "a written draft reads back", writes: []string{"---\ntitle: Add retry\n---\n"}, wantContent: "---\ntitle: Add retry\n---\n", wantIsFound: true},
		{name: "a second write replaces the draft", writes: []string{"first", "second"}, wantContent: "second", wantIsFound: true},
		{name: "a removed draft is not found", writes: []string{"first"}, isRemoved: true},
		{name: "removing no draft is no error", isRemoved: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := NewFileDraftRepository(t.TempDir())
			for _, content := range testCase.writes {
				if err := repo.WriteDraft(draftTestProject, task.NewTaskDraftName, content); err != nil {
					t.Fatalf("WriteDraft: %v", err)
				}
			}
			if testCase.isRemoved {
				if err := repo.RemoveDraft(draftTestProject, task.NewTaskDraftName); err != nil {
					t.Fatalf("RemoveDraft: %v", err)
				}
			}

			content, isFound, err := repo.ReadDraft(draftTestProject, task.NewTaskDraftName)

			if err != nil {
				t.Fatalf("ReadDraft: %v", err)
			}
			if content != testCase.wantContent || isFound != testCase.wantIsFound {
				t.Errorf("expected %q, found %v, got %q, found %v", testCase.wantContent, testCase.wantIsFound, content, isFound)
			}
		})
	}
}

func TestFileDraftRepository_WriteDraftOnDisk(t *testing.T) {
	repo := NewFileDraftRepository(t.TempDir())
	if err := repo.WriteDraft(draftTestProject, task.NewTaskDraftName, "Retry the upload."); err != nil {
		t.Fatalf("WriteDraft: %v", err)
	}
	path, err := repo.DraftPath(draftTestProject, task.NewTaskDraftName)
	if err != nil {
		t.Fatalf("DraftPath: %v", err)
	}

	content, err := os.ReadFile(path)

	if err != nil || string(content) != "Retry the upload." {
		t.Errorf("expected the draft at %s, got %q, %v", path, content, err)
	}
}
