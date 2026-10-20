package task

import (
	"errors"
	"fmt"
	"io/fs"
	"time"
)

// ArchiveFormatVersion is the version of the archive layout ExportTasks
// writes into the manifest.
const ArchiveFormatVersion = 1

// archiveNameFormat names the archive ExportTasks writes when the caller gives
// no path. It takes the project slug, the task count and the export time.
const archiveNameFormat = "%s-%d-tasks-%s.zip"

// archiveTimeLayout renders the export time in a default archive name.
const archiveTimeLayout = "20060102-150405"

// TaskFile is one task file as it sits on disk, along with the task parsed out
// of it.
type TaskFile struct {
	Name    string
	Content []byte
	Task    *Task
}

// ArchiveManifest describes an archive of task files.
type ArchiveManifest struct {
	FormatVersion int
	ProjectSlug   string
	ExportedAt    time.Time
}

// ExportTasksDto names the project to export and where the archive goes. An
// empty Path writes the default archive name in the current directory. A zero
// ExportedAt means now.
type ExportTasksDto struct {
	ProjectSlug string
	Path        string
	ExportedAt  time.Time
}

// ExportedTasks is the archive ExportTasks wrote.
type ExportedTasks struct {
	Path  string
	Count int
}

// ExportTasks writes every task file of a project, whatever its status, into
// a new archive along with a manifest. It parses every task file before it
// writes anything, and refuses a project with no tasks and a path that already
// exists.
func (service *TaskService) ExportTasks(dto ExportTasksDto) (ExportedTasks, error) {
	if dto.ProjectSlug == "" {
		return ExportedTasks{}, errors.New("project slug is required")
	}
	if dto.ExportedAt.IsZero() {
		dto.ExportedAt = time.Now().UTC().Truncate(time.Second)
	}

	files, err := service.repo.ReadTaskFiles(dto.ProjectSlug)
	if err != nil {
		return ExportedTasks{}, fmt.Errorf("could not export the tasks of project %s: %w", dto.ProjectSlug, err)
	}
	if len(files) == 0 {
		return ExportedTasks{}, fmt.Errorf("project %s has no tasks to export", dto.ProjectSlug)
	}

	path := dto.Path
	if path == "" {
		path = fmt.Sprintf(archiveNameFormat, dto.ProjectSlug, len(files), dto.ExportedAt.Format(archiveTimeLayout))
	}

	manifest := ArchiveManifest{FormatVersion: ArchiveFormatVersion, ProjectSlug: dto.ProjectSlug, ExportedAt: dto.ExportedAt}
	err = service.repo.WriteTaskArchive(path, manifest, files)
	if errors.Is(err, fs.ErrExist) {
		return ExportedTasks{}, fmt.Errorf("%s already exists, remove it or export to another path", path)
	}
	if err != nil {
		return ExportedTasks{}, fmt.Errorf("could not write the archive %s: %w", path, err)
	}
	return ExportedTasks{Path: path, Count: len(files)}, nil
}
