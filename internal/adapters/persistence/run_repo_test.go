package persistence

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const runTestTaskID task.TaskID = "task-1"

const runTestStream = `{"type":"system","subtype":"init"}` + "\n" + `{"type":"result","subtype":"success"}` + "\n"

// runFiles are the files a test puts in the run directory, keyed by their name.
// A nil map stands for a task with no run directory.
type runFiles map[string]string

// setupRunRepo builds a repository over a temp project directory holding the
// files of one run, and returns it with the run directory.
func setupRunRepo(t *testing.T, files runFiles) (*FileRunRepository, string) {
	t.Helper()
	projectDir := t.TempDir()
	runDir := common.RunDir(projectDir, string(runTestTaskID))

	if files != nil {
		if err := common.EnsureDir(runDir); err != nil {
			t.Fatalf("could not create the run directory: %v", err)
		}
	}
	for name, content := range files {
		path := filepath.Join(runDir, name)
		if err := common.EnsureDir(filepath.Dir(path)); err != nil {
			t.Fatalf("could not create the directory of %s: %v", name, err)
		}
		if err := common.WriteFile(path, content); err != nil {
			t.Fatalf("could not write %s: %v", name, err)
		}
	}
	return NewFileRunRepository(projectDir), runDir
}

func TestFileRunRepository_PrepareRun(t *testing.T) {
	cases := []struct {
		name  string
		files runFiles
	}{
		{name: "a task with no run"},
		{
			name: "a task whose earlier run finished",
			files: runFiles{
				common.RunPromptName: "the old prompt",
				common.RunStreamName: runTestStream,
				common.RunStderrName: "boom\n",
				common.RunExitName:   "1\n",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, runDir := setupRunRepo(t, testCase.files)

			if err := repo.PrepareRun(runTestTaskID, "the new prompt"); err != nil {
				t.Fatalf("PrepareRun: %v", err)
			}

			entries, err := os.ReadDir(runDir)
			if err != nil {
				t.Fatalf("could not list the run directory: %v", err)
			}
			if len(entries) != 1 || entries[0].Name() != common.RunPromptName {
				t.Errorf("expected the run directory to hold only %s, got %v", common.RunPromptName, entries)
			}
			prompt, err := common.ReadFile(common.RunPromptPath(runDir))
			if err != nil {
				t.Fatalf("could not read the prompt: %v", err)
			}
			if prompt != "the new prompt" {
				t.Errorf("expected the new prompt, got %q", prompt)
			}
		})
	}
}

func TestFileRunRepository_HasRun(t *testing.T) {
	cases := []struct {
		name  string
		files runFiles
		want  bool
	}{
		{name: "a task with no run"},
		{name: "an empty run directory", files: runFiles{}, want: true},
		{name: "a prepared run", files: runFiles{common.RunPromptName: "prompt"}, want: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, _ := setupRunRepo(t, testCase.files)

			got, err := repo.HasRun(runTestTaskID)
			if err != nil {
				t.Fatalf("HasRun: %v", err)
			}
			if got != testCase.want {
				t.Errorf("expected a run %v, got %v", testCase.want, got)
			}
		})
	}
}

// TestFileRunRepository_ReadFiles covers every method that reads one file of a
// run and reports whether the file is there.
func TestFileRunRepository_ReadFiles(t *testing.T) {
	readers := []struct {
		name     string
		fileName string
		read     func(repo *FileRunRepository) (string, bool, error)
	}{
		{
			name:     "ReadStream",
			fileName: common.RunStreamName,
			read: func(repo *FileRunRepository) (string, bool, error) {
				content, isPresent, err := repo.ReadStream(runTestTaskID)
				return string(content), isPresent, err
			},
		},
		{
			name:     "ReadStderr",
			fileName: common.RunStderrName,
			read: func(repo *FileRunRepository) (string, bool, error) {
				content, isPresent, err := repo.ReadStderr(runTestTaskID)
				return string(content), isPresent, err
			},
		},
		{
			name:     "ReadExit",
			fileName: common.RunExitName,
			read: func(repo *FileRunRepository) (string, bool, error) {
				return repo.ReadExit(runTestTaskID)
			},
		},
	}

	cases := []struct {
		name string
		// content is what the file holds. A nil one stands for a file that is
		// not there, and isRunMissing for a task with no run directory at all.
		content      *string
		isRunMissing bool
		wantPresent  bool
	}{
		{name: "a missing run", isRunMissing: true},
		{name: "a missing file"},
		{name: "an empty file", content: ptr(""), wantPresent: true},
		{name: "a file with content", content: ptr(runTestStream), wantPresent: true},
		{name: "an exit code", content: ptr("0\n"), wantPresent: true},
	}

	for _, reader := range readers {
		for _, testCase := range cases {
			t.Run(reader.name+"/"+testCase.name, func(t *testing.T) {
				files := runFiles{common.RunPromptName: "prompt"}
				if testCase.isRunMissing {
					files = nil
				}
				if testCase.content != nil {
					files[reader.fileName] = *testCase.content
				}
				repo, _ := setupRunRepo(t, files)

				got, isPresent, err := reader.read(repo)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if isPresent != testCase.wantPresent {
					t.Errorf("expected the file present %v, got %v", testCase.wantPresent, isPresent)
				}
				want := ""
				if testCase.content != nil {
					want = *testCase.content
				}
				if got != want {
					t.Errorf("expected %q, got %q", want, got)
				}
			})
		}
	}
}

func TestFileRunRepository_ReadStream_UnreadableStream(t *testing.T) {
	repo, runDir := setupRunRepo(t, runFiles{})
	// A directory standing where the stream file belongs is not a missing
	// file, so the read has to fail.
	if err := os.Mkdir(common.RunStreamPath(runDir), 0o755); err != nil {
		t.Fatalf("could not create the stand-in stream: %v", err)
	}

	if _, _, err := repo.ReadStream(runTestTaskID); err == nil {
		t.Fatal("expected an error for a stream that cannot be read")
	}
}

func TestFileRunRepository_LastWrite(t *testing.T) {
	streamWrittenAt := time.Now().Add(-time.Hour).Truncate(time.Second)
	runPreparedAt := time.Now().Add(-2 * time.Hour).Truncate(time.Second)

	cases := []struct {
		name    string
		files   runFiles
		want    time.Time
		wantErr bool
	}{
		{name: "a missing run", wantErr: true},
		{name: "a run with no stream", files: runFiles{}, want: runPreparedAt},
		{name: "an empty stream", files: runFiles{common.RunStreamName: ""}, want: streamWrittenAt},
		{name: "a stream with events", files: runFiles{common.RunStreamName: runTestStream}, want: streamWrittenAt},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, runDir := setupRunRepo(t, testCase.files)
			if testCase.files != nil {
				setModTime(t, runDir, runPreparedAt)
			}
			if _, hasStream := testCase.files[common.RunStreamName]; hasStream {
				setModTime(t, common.RunStreamPath(runDir), streamWrittenAt)
			}

			got, err := repo.LastWrite(runTestTaskID)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("LastWrite: %v", err)
			}
			if !got.Equal(testCase.want) {
				t.Errorf("expected %v, got %v", testCase.want, got)
			}
		})
	}
}

func TestFileRunRepository_RemoveRun(t *testing.T) {
	cases := []struct {
		name  string
		files runFiles
		want  bool
	}{
		{name: "a task with no run"},
		{name: "a finished run", files: runFiles{common.RunStreamName: runTestStream, common.RunExitName: "0\n"}, want: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, runDir := setupRunRepo(t, testCase.files)

			got, err := repo.RemoveRun(runTestTaskID)
			if err != nil {
				t.Fatalf("RemoveRun: %v", err)
			}
			if got != testCase.want {
				t.Errorf("expected a removed run %v, got %v", testCase.want, got)
			}

			isLeft, err := common.Exists(runDir)
			if err != nil {
				t.Fatalf("could not check the run directory: %v", err)
			}
			if isLeft {
				t.Error("expected the run directory to be gone")
			}
		})
	}
}

func setModTime(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("could not set the modification time of %s: %v", path, err)
	}
}

func ptr(value string) *string {
	return &value
}

func TestFileRunRepository_PullRequests(t *testing.T) {
	const (
		repository  = "api"
		description = "Add retry\n\nRetries uploads."
	)

	cases := []struct {
		name  string
		files runFiles
		// change runs against the repository before the description is read.
		change          func(repo *FileRunRepository) error
		want            string
		wantPresent     bool
		wantErrContains string
	}{
		{
			name:  "a run with no description",
			files: runFiles{},
		},
		{
			name:  "a written description",
			files: runFiles{},
			change: func(repo *FileRunRepository) error {
				return repo.WritePullRequest(runTestTaskID, repository, description)
			},
			want:        description,
			wantPresent: true,
		},
		{
			name:  "a removed description",
			files: runFiles{},
			change: func(repo *FileRunRepository) error {
				if err := repo.WritePullRequest(runTestTaskID, repository, description); err != nil {
					return err
				}
				return repo.RemovePullRequest(runTestTaskID, repository)
			},
		},
		{
			name:  "removing a description that is not there",
			files: runFiles{},
			change: func(repo *FileRunRepository) error {
				return repo.RemovePullRequest(runTestTaskID, repository)
			},
		},
		{
			name: "writing to a task with no run",
			change: func(repo *FileRunRepository) error {
				return repo.WritePullRequest(runTestTaskID, repository, description)
			},
			wantErrContains: "has no run directory",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, runDir := setupRunRepo(t, testCase.files)

			if testCase.change != nil {
				err := testCase.change(repo)
				if testCase.wantErrContains != "" {
					if err == nil || !strings.Contains(err.Error(), testCase.wantErrContains) {
						t.Fatalf("expected an error naming %q, got %v", testCase.wantErrContains, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			got, isPresent, err := repo.ReadPullRequest(runTestTaskID, repository)
			if err != nil {
				t.Fatalf("ReadPullRequest: %v", err)
			}
			if isPresent != testCase.wantPresent || got != testCase.want {
				t.Errorf("ReadPullRequest() = %q, %t, want %q, %t", got, isPresent, testCase.want, testCase.wantPresent)
			}
			if testCase.wantPresent {
				onDisk, err := common.ReadFile(filepath.Join(runDir, common.RunPullRequestsDirName, repository+".md"))
				if err != nil {
					t.Fatalf("expected the description in the run directory: %v", err)
				}
				if onDisk != description {
					t.Errorf("expected %q on disk, got %q", description, onDisk)
				}
			}
		})
	}
}

func TestFileRunRepository_ListPullRequests(t *testing.T) {
	cases := []struct {
		name  string
		files runFiles
		want  []string
	}{
		{name: "a task with no run"},
		{name: "a run with no descriptions", files: runFiles{common.RunPromptName: "prompt"}},
		{
			name: "descriptions of two repositories",
			files: runFiles{
				filepath.Join(common.RunPullRequestsDirName, "ui.md"):  "Show retries",
				filepath.Join(common.RunPullRequestsDirName, "api.md"): "Add retry",
			},
			want: []string{"api", "ui"},
		},
		{
			name: "a file that is not a description",
			files: runFiles{
				filepath.Join(common.RunPullRequestsDirName, "api.md"):    "Add retry",
				filepath.Join(common.RunPullRequestsDirName, "notes.txt"): "scratch",
			},
			want: []string{"api"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, _ := setupRunRepo(t, testCase.files)

			got, err := repo.ListPullRequests(runTestTaskID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, testCase.want) {
				t.Errorf("ListPullRequests() = %v, want %v", got, testCase.want)
			}
		})
	}
}
