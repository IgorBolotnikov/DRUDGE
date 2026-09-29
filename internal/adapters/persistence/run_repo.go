package persistence

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// FileRunRepository keeps the run of each task in a run directory inside the
// project directory. The launcher script writes the stream, the stderr log and
// the exit file to the paths common.Run*Path gives, so this repository reads
// them from there.
type FileRunRepository struct {
	// projectDir is the directory the run directories live under. An empty one
	// stands for the working directory. This attribute is mostly for testing
	// purposes.
	projectDir string
}

func NewFileRunRepository(projectDir string) *FileRunRepository {
	return &FileRunRepository{projectDir: projectDir}
}

// PrepareRun deletes the run directory of a task and creates it again holding
// only the prompt.
func (repo *FileRunRepository) PrepareRun(taskID task.TaskID, prompt string) error {
	runDir, err := repo.runDir(taskID)
	if err != nil {
		return err
	}
	if err := common.RemoveAll(runDir); err != nil {
		return err
	}
	if err := common.EnsureDir(runDir); err != nil {
		return err
	}
	return common.WriteFile(common.RunPromptPath(runDir), prompt)
}

// HasRun tells whether a task has a run directory.
func (repo *FileRunRepository) HasRun(taskID task.TaskID) (bool, error) {
	runDir, err := repo.runDir(taskID)
	if err != nil {
		return false, err
	}
	return common.Exists(runDir)
}

// ReadStream reads the event stream file of a run.
func (repo *FileRunRepository) ReadStream(taskID task.TaskID) ([]byte, bool, error) {
	runDir, err := repo.runDir(taskID)
	if err != nil {
		return nil, false, err
	}
	return readRunFile(common.RunStreamPath(runDir))
}

// LastWrite returns the modification time of the event stream file, or of the
// run directory when the stream file does not exist.
func (repo *FileRunRepository) LastWrite(taskID task.TaskID) (time.Time, error) {
	runDir, err := repo.runDir(taskID)
	if err != nil {
		return time.Time{}, err
	}

	stream, err := os.Stat(common.RunStreamPath(runDir))
	if err == nil {
		return stream.ModTime(), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, fmt.Errorf("could not check the event stream of run directory %s: %w", runDir, err)
	}

	directory, err := os.Stat(runDir)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not check run directory %s: %w", runDir, err)
	}
	return directory.ModTime(), nil
}

// ReadExit reads the exit file of a run.
func (repo *FileRunRepository) ReadExit(taskID task.TaskID) (string, bool, error) {
	runDir, err := repo.runDir(taskID)
	if err != nil {
		return "", false, err
	}
	content, isPresent, err := readRunFile(common.RunExitPath(runDir))
	return string(content), isPresent, err
}

// ReadStderr reads the stderr log file of a run.
func (repo *FileRunRepository) ReadStderr(taskID task.TaskID) ([]byte, bool, error) {
	runDir, err := repo.runDir(taskID)
	if err != nil {
		return nil, false, err
	}
	return readRunFile(common.RunStderrPath(runDir))
}

// RemoveRun deletes the run directory of a task.
func (repo *FileRunRepository) RemoveRun(taskID task.TaskID) (bool, error) {
	runDir, err := repo.runDir(taskID)
	if err != nil {
		return false, err
	}

	hasRun, err := common.Exists(runDir)
	if err != nil {
		return false, err
	}
	if !hasRun {
		return false, nil
	}

	if err := common.RemoveAll(runDir); err != nil {
		return false, err
	}
	return true, nil
}

func (repo *FileRunRepository) runDir(taskID task.TaskID) (string, error) {
	projectDir := repo.projectDir
	if projectDir == "" {
		workDir, err := common.WorkDir()
		if err != nil {
			return "", err
		}
		projectDir = workDir
	}
	return common.RunDir(projectDir, string(taskID)), nil
}

// readRunFile reads one file of a run directory. isPresent is false when the
// file does not exist.
func readRunFile(path string) (content []byte, isPresent bool, err error) {
	content, err = os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("could not read %s: %w", path, err)
	}
	return content, true, nil
}
