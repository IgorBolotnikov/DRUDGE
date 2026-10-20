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

// TaskImportUpdated reports a task ImportTasks replaced with a newer copy from
// the archive.
type TaskImportUpdated struct {
	Task *Task
}

// ImportSkipReason says why ImportTasks kept the copy of a task the project
// has.
type ImportSkipReason string

const (
	// SkipReasonOlder means the archived copy has the same or an older update
	// time.
	SkipReasonOlder ImportSkipReason = "older"
	// SkipReasonInProgress means the task is in progress in the project. A
	// Drudger on this device holds it.
	SkipReasonInProgress ImportSkipReason = "in-progress"
)

// TaskImportSkipped reports a task of the archive ImportTasks did not write
// over the copy the project has.
type TaskImportSkipped struct {
	Task   *Task
	Reason ImportSkipReason
}

// TasksImported reports how an import ended. It is the last event of an
// import.
type TasksImported struct {
	ProjectSlug         string
	Added               int
	Updated             int
	SkippedAsOlder      int
	SkippedAsInProgress int
}

// ImportTasks adds every task of an archive the project does not have yet, and
// replaces every task the archive holds a newer copy of, as it is in the
// archive. A task in progress in the project is never replaced. A task keeps
// its id, its timestamps and its run state, and only its project slug is set
// to the target project. It reads and parses
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

		var isStored bool
		var skipReason ImportSkipReason
		isWritten, err := service.repo.WriteTask(dto.ProjectSlug, archived, func(stored *Task) error {
			isStored = stored != nil
			skipReason = resolveImport(stored, archived)
			if skipReason != "" {
				return ErrTaskUnchanged
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not import task %s: %w", archived.ID, err)
		}

		switch {
		case isWritten && isStored:
			summary.Updated++
			service.progress.Report(TaskImportUpdated{Task: archived})
		case isWritten:
			summary.Added++
			service.progress.Report(TaskImported{Task: archived})
		case skipReason == SkipReasonInProgress:
			summary.SkippedAsInProgress++
			service.progress.Report(TaskImportSkipped{Task: archived, Reason: skipReason})
		default:
			summary.SkippedAsOlder++
			service.progress.Report(TaskImportSkipped{Task: archived, Reason: skipReason})
		}
	}

	service.progress.Report(summary)
	return nil
}

// resolveImport decides whether an archived task replaces the stored copy of
// it. stored is nil when the project has no task with its id. It returns why
// the stored copy stays, or an empty reason when the archived task is written.
func resolveImport(stored *Task, archived *Task) ImportSkipReason {
	switch {
	case stored == nil:
		return ""
	case stored.Status == StatusInProgress:
		return SkipReasonInProgress
	case archived.UpdatedAt.After(stored.UpdatedAt):
		return ""
	default:
		return SkipReasonOlder
	}
}
