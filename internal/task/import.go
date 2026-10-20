package task

import (
	"errors"
	"fmt"
)

// freshExportAdvice tells the user how to get past an archive import cannot
// read.
const freshExportAdvice = "a fresh export on the source device fixes it"

// BrokenArchiveError reports an archive import cannot read.
type BrokenArchiveError struct {
	Reason string
}

func (err *BrokenArchiveError) Error() string {
	return err.Reason
}

// ImportTasksDto names the archive to import and the project it goes into.
type ImportTasksDto struct {
	ProjectSlug string
	Path        string
}

// ProjectSlugRewritten warns that an archive holds the tasks of another
// project. ImportTasks moves every task into the target project.
type ProjectSlugRewritten struct {
	ArchiveSlug string
	ProjectSlug string
}

// TaskImported reports a task ImportTasks added to the project.
type TaskImported struct {
	Task *Task
}

// TaskImportSkipped reports a task ImportTasks left alone, because the project
// has a task with its id.
type TaskImportSkipped struct {
	Task *Task
}

// TasksImported reports how an import ended. It is the last event of an
// import.
type TasksImported struct {
	ProjectSlug string
	Added       int
	Skipped     int
}

// ImportTasks adds every task of an archive the project does not have yet, as
// it is in the archive. A task keeps its id, its timestamps and its run state,
// and only its project slug is set to the target project. It reads and parses
// the whole archive before it writes anything, and refuses an archive of
// another format version.
func (service *TaskService) ImportTasks(dto ImportTasksDto) error {
	if dto.ProjectSlug == "" {
		return errors.New("project slug is required")
	}
	if dto.Path == "" {
		return errors.New("archive path is required")
	}

	manifest, files, err := service.repo.ReadTaskArchive(dto.Path)
	var broken *BrokenArchiveError
	if errors.As(err, &broken) {
		return fmt.Errorf("could not import %s: %w, %s", dto.Path, err, freshExportAdvice)
	}
	if err != nil {
		return fmt.Errorf("could not import %s: %w", dto.Path, err)
	}
	if manifest.FormatVersion != ArchiveFormatVersion {
		return fmt.Errorf(
			"could not import %s: it has archive format version %d and drg reads version %d, %s",
			dto.Path, manifest.FormatVersion, ArchiveFormatVersion, freshExportAdvice,
		)
	}

	if manifest.ProjectSlug != dto.ProjectSlug {
		service.progress.Report(ProjectSlugRewritten{ArchiveSlug: manifest.ProjectSlug, ProjectSlug: dto.ProjectSlug})
	}

	summary := TasksImported{ProjectSlug: dto.ProjectSlug}
	for _, file := range files {
		archived := file.Task
		archived.ProjectSlug = dto.ProjectSlug

		isWritten, err := service.repo.WriteTask(dto.ProjectSlug, archived, func(stored *Task) error {
			return resolveImport(stored, archived)
		})
		if err != nil {
			return fmt.Errorf("could not import task %s: %w", archived.ID, err)
		}

		if isWritten {
			summary.Added++
			service.progress.Report(TaskImported{Task: archived})
		} else {
			summary.Skipped++
			service.progress.Report(TaskImportSkipped{Task: archived})
		}
	}

	service.progress.Report(summary)
	return nil
}

// resolveImport decides whether an archived task replaces the stored copy of
// it. stored is nil when the project has no task with its id. It returns
// ErrTaskUnchanged to keep the stored copy.
func resolveImport(stored *Task, archived *Task) error {
	if stored != nil {
		return ErrTaskUnchanged
	}
	return nil
}
