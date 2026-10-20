package persistence

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// ManifestFileName is the name of the manifest at the root of a task archive.
const ManifestFileName = "manifest.json"

// archiveManifest is the manifest of a task archive as it is stored.
type archiveManifest struct {
	FormatVersion int       `json:"formatVersion"`
	ProjectSlug   string    `json:"projectSlug"`
	ExportedAt    time.Time `json:"exportedAt"`
}

// ReadTaskFiles reads every task file of the project as it is stored. Each
// file is read under the lock of its task, and a task removed before its lock
// was taken is left out. A project with no tasks directory has no task files.
func (r *FileTaskRepository) ReadTaskFiles(projectSlug string) ([]task.TaskFile, error) {
	isPresent, err := common.Exists(r.taskDir())
	if err != nil {
		return nil, fmt.Errorf("could not check tasks directory: %w", err)
	}
	if !isPresent {
		return nil, nil
	}

	listed, err := r.taskFiles()
	if err != nil {
		return nil, err
	}

	files := make([]task.TaskFile, 0, len(listed))
	for _, candidate := range listed {
		read, isPresent, err := r.readTaskFileLocked(candidate)
		if err != nil {
			return nil, err
		}
		if isPresent {
			files = append(files, read)
		}
	}
	return files, nil
}

// readTaskFileLocked reads one task file under the lock of its task. isPresent
// is false when the task was removed before the lock was taken.
func (r *FileTaskRepository) readTaskFileLocked(listed taskFile) (read task.TaskFile, isPresent bool, err error) {
	if listed.id == "" {
		return task.TaskFile{}, false, fmt.Errorf("task file %s has no task id at the start of its name", listed.path)
	}

	unlock, _, err := lockFile(r.taskLockPath(listed.id), waitForLock)
	if err != nil {
		return task.TaskFile{}, false, err
	}
	defer unlock()

	// The task file is named after the task title, so its path is looked up
	// again under the lock.
	files, err := r.taskFiles()
	if err != nil {
		return task.TaskFile{}, false, err
	}
	index := slices.IndexFunc(files, func(candidate taskFile) bool { return candidate.id == listed.id })
	if index < 0 {
		return task.TaskFile{}, false, nil
	}
	found := files[index]

	data, err := os.ReadFile(found.path)
	if err != nil {
		return task.TaskFile{}, false, fmt.Errorf("could not read file %s: %w", found.path, err)
	}
	parsed, err := parseTaskFile(found, data)
	if err != nil {
		return task.TaskFile{}, false, err
	}
	return task.TaskFile{Name: filepath.Base(found.path), Content: data, Task: parsed}, true, nil
}

// WriteTaskArchive writes a zip archive at path holding every task file at its
// root and the manifest beside them. It refuses a path that already exists,
// and removes what it wrote when the write fails.
func (r *FileTaskRepository) WriteTaskArchive(path string, manifest task.ArchiveManifest, files []task.TaskFile) (err error) {
	archiveFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, common.DefaultFilePerm)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(path)
		}
	}()

	if err := writeZip(archiveFile, manifest, files); err != nil {
		archiveFile.Close()
		return err
	}
	return archiveFile.Close()
}

// writeZip writes the task files and the manifest into a zip archive. Every
// entry carries the export time.
func writeZip(destination io.Writer, manifest task.ArchiveManifest, files []task.TaskFile) error {
	archive := zip.NewWriter(destination)

	for _, file := range files {
		if err := writeZipEntry(archive, file.Name, manifest.ExportedAt, file.Content); err != nil {
			return err
		}
	}

	manifestJSON, err := json.MarshalIndent(archiveManifest{
		FormatVersion: manifest.FormatVersion,
		ProjectSlug:   manifest.ProjectSlug,
		ExportedAt:    manifest.ExportedAt,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode the manifest: %w", err)
	}
	if err := writeZipEntry(archive, ManifestFileName, manifest.ExportedAt, manifestJSON); err != nil {
		return err
	}

	return archive.Close()
}

func writeZipEntry(archive *zip.Writer, name string, modified time.Time, content []byte) error {
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: modified})
	if err != nil {
		return fmt.Errorf("could not add %s to the archive: %w", name, err)
	}
	if _, err := entry.Write(content); err != nil {
		return fmt.Errorf("could not add %s to the archive: %w", name, err)
	}
	return nil
}
