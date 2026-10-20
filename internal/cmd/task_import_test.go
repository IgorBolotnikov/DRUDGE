package cmd

import (
	"archive/zip"
	"os"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

func TestTaskImport(t *testing.T) {
	const (
		manifest  = `{"formatVersion":1,"projectSlug":"demo","exportedAt":"2026-10-18T14:30:05Z"}`
		loginFile = "006684e3-dbe9-4316-8aba-8a67a8f01f8f Fix login.md"
		loginTask = "---\nid: 006684e3-dbe9-4316-8aba-8a67a8f01f8f\nstatus: todo\ntitle: Fix login\n---\n"
	)

	cases := []struct {
		name    string
		entries map[string]string
		// wantErr is the error the command ends with, empty for none.
		wantErr      string
		wantWarnings int
	}{
		{
			name:    "an archive with no bad entries",
			entries: map[string]string{persistence.ManifestFileName: manifest, loginFile: loginTask},
		},
		{
			name:         "an archive with bad entries",
			entries:      map[string]string{persistence.ManifestFileName: manifest, loginFile: loginTask, "notes.txt": "Ship it on Friday", "../escape.md": loginTask},
			wantErr:      "skipped 2 entries of tasks.zip as bad, a fresh export on the source device fixes them",
			wantWarnings: 2,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			t.Chdir(t.TempDir())
			service := project.NewProjectService(persistence.NewFileProjectRepository(""), nil, nil, nil, nil, newTestCLIProgress())
			captureOutput(func() {
				if _, err := service.CreateProject("demo"); err != nil {
					t.Fatal(err)
				}
			})
			writeTestArchive(t, "tasks.zip", testCase.entries)

			var err error
			stderr := captureStderr(func() {
				captureOutput(func() { err = NewRoot("v1.2.3").Execute([]string{TaskCmd.Name, "import", "-p", "demo", "tasks.zip"}) })
			})

			if testCase.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if testCase.wantErr != "" && (err == nil || err.Error() != testCase.wantErr) {
				t.Fatalf("error = %v, want %q", err, testCase.wantErr)
			}
			if warnings := strings.Count(stderr, "! Skipped "); warnings != testCase.wantWarnings {
				t.Errorf("expected %d warnings about bad entries, got stderr %q", testCase.wantWarnings, stderr)
			}
			tasks, err := persistence.NewFileTaskRepository("demo").ListTasks("demo")
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != 1 || tasks[0].Title != "Fix login" {
				t.Errorf("expected only task Fix login, got %+v", tasks)
			}
		})
	}
}

// writeTestArchive writes a zip archive holding entries at path.
func writeTestArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	archiveFile, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archiveFile.Close()

	archive := zip.NewWriter(archiveFile)
	for name, content := range entries {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
}
