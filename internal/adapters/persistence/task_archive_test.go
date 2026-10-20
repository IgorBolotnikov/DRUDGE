package persistence

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const (
	loginTaskFile = "006684e3-dbe9-4316-8aba-8a67a8f01f8f Fix login.md"
	loginTask     = "---\nid: 006684e3-dbe9-4316-8aba-8a67a8f01f8f\nstatus: done\ntitle: Fix login\n---\nUsers can't log in with SSO\n"
	shipTaskFile  = "abcd1234-2222-4316-8aba-8a67a8f01f8f Ship it.md"
	shipTask      = "---\nid: abcd1234-2222-4316-8aba-8a67a8f01f8f\nstatus: draft\ntitle: Ship it\nunknown_key: kept as is\n---\n"
)

// writeRawTaskFiles writes files into the tasks directory of the test project
// byte for byte.
func writeRawTaskFiles(t *testing.T, home string, files map[string]string) {
	t.Helper()
	tasksDir := filepath.Join(common.ProjectsDir(home), "test-project", TasksDirName)
	if err := common.EnsureDir(tasksDir); err != nil {
		t.Fatalf("ensure tasks dir: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tasksDir, name), []byte(content), common.DefaultFilePerm); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func TestFileTaskRepository_ReadTaskFiles(t *testing.T) {
	cases := []struct {
		name string
		// files are the files of the tasks directory. A nil map leaves the
		// project with no tasks directory.
		files     map[string]string
		wantFiles map[string]task.TaskStatus
		// wantErrText is a fragment a refusal must carry.
		wantErrText string
	}{
		{
			name:      "a few tasks",
			files:     map[string]string{loginTaskFile: loginTask, shipTaskFile: shipTask, "notes.txt": "not a task"},
			wantFiles: map[string]task.TaskStatus{loginTaskFile: task.StatusDone, shipTaskFile: task.StatusDraft},
		},
		{
			name:      "a project with no tasks directory",
			wantFiles: map[string]task.TaskStatus{},
		},
		{
			name:        "a file holding another task",
			files:       map[string]string{loginTaskFile: loginTask, "99999999-0000-4000-8000-000000000000 Fix login.md": loginTask},
			wantErrText: "99999999-0000-4000-8000-000000000000 Fix login.md holds task 006684e3-dbe9-4316-8aba-8a67a8f01f8f",
		},
		{
			name:        "a file with no front matter",
			files:       map[string]string{shipTaskFile: shipTask, "12345678 Notes.md": "just some notes\n"},
			wantErrText: `12345678 Notes.md has no "id" in its front matter`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home, cleanup := setupTaskTestHome(t)
			defer cleanup()
			if testCase.files != nil {
				writeRawTaskFiles(t, home, testCase.files)
			}

			files, err := NewFileTaskRepository("test-project").ReadTaskFiles("test-project")

			if testCase.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Fatalf("expected an error containing %q, got %v", testCase.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotFiles := map[string]task.TaskStatus{}
			for _, file := range files {
				gotFiles[file.Name] = file.Task.Status
				if string(file.Content) != testCase.files[file.Name] {
					t.Errorf("expected %s read byte for byte, got %q", file.Name, file.Content)
				}
			}
			if !reflect.DeepEqual(gotFiles, testCase.wantFiles) {
				t.Errorf("expected files %v, got %v", testCase.wantFiles, gotFiles)
			}
		})
	}
}

func TestFileTaskRepository_WriteTaskArchive(t *testing.T) {
	exportedAt := time.Date(2026, 10, 18, 14, 30, 5, 0, time.UTC)
	manifest := task.ArchiveManifest{FormatVersion: 1, ProjectSlug: "test-project", ExportedAt: exportedAt}
	files := []task.TaskFile{
		{Name: loginTaskFile, Content: []byte(loginTask)},
		{Name: shipTaskFile, Content: []byte(shipTask)},
	}

	cases := []struct {
		name        string
		isPathTaken bool
	}{
		{name: "a new path"},
		{name: "a path that already exists", isPathTaken: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tasks.zip")
			const takenContent = "someone else's file"
			if testCase.isPathTaken {
				if err := os.WriteFile(path, []byte(takenContent), common.DefaultFilePerm); err != nil {
					t.Fatal(err)
				}
			}

			err := NewFileTaskRepository("test-project").WriteTaskArchive(path, manifest, files)

			if testCase.isPathTaken {
				if !errors.Is(err, fs.ErrExist) {
					t.Fatalf("expected an error matching fs.ErrExist, got %v", err)
				}
				if content, _ := os.ReadFile(path); string(content) != takenContent {
					t.Errorf("expected the existing file left alone, got %q", content)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantEntries := map[string]string{
				loginTaskFile:    loginTask,
				shipTaskFile:     shipTask,
				ManifestFileName: `{"formatVersion":1,"projectSlug":"test-project","exportedAt":"2026-10-18T14:30:05Z"}`,
			}
			gotEntries := readZipEntries(t, path)
			var gotManifest, wantManifest any
			if err := json.Unmarshal([]byte(gotEntries[ManifestFileName]), &gotManifest); err != nil {
				t.Fatalf("decode manifest: %v", err)
			}
			if err := json.Unmarshal([]byte(wantEntries[ManifestFileName]), &wantManifest); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotManifest, wantManifest) {
				t.Errorf("expected manifest %v, got %v", wantManifest, gotManifest)
			}
			delete(gotEntries, ManifestFileName)
			delete(wantEntries, ManifestFileName)
			if !reflect.DeepEqual(gotEntries, wantEntries) {
				t.Errorf("expected entries %v, got %v", wantEntries, gotEntries)
			}
		})
	}
}

// readZipEntries reads the name and the content of every entry of a zip
// archive.
func readZipEntries(t *testing.T, path string) map[string]string {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer archive.Close()

	entries := map[string]string{}
	for _, entry := range archive.File {
		reader, err := entry.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry.Name, err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name, err)
		}
		entries[entry.Name] = string(content)
	}
	return entries
}
