package task

import (
	"errors"
	"fmt"
	"slices"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
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

// TaskArchive is what an archive holds. BadEntries are the entries that are
// neither a task file that parses nor the manifest.
type TaskArchive struct {
	Manifest   ArchiveManifest
	Files      []TaskFile
	BadEntries []BadArchiveEntry
}

// BadArchiveEntry warns about an entry of an archive ImportTasks skips, and
// says why.
type BadArchiveEntry struct {
	Name   string
	Reason string
}

// ImportTasksDto names the archive to import and the project it goes into.
type ImportTasksDto struct {
	ProjectSlug string
	Path        string
	// IsDryRun runs every check and comparison of an import and writes
	// nothing.
	IsDryRun bool
}

// ImportDryRunStarted reports that an import is a dry run. It comes before
// every other event of the import.
type ImportDryRunStarted struct {
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
	SkippedAsBad        int
	IsDryRun            bool
}

// ImportTasks adds every task of an archive the project does not have yet, and
// replaces every task the archive holds a newer copy of, as it is in the
// archive. A task in progress in the project is never replaced. A task keeps
// its id, its timestamps and its run state, and only its project slug is set
// to the target project. It reads and checks the whole archive before it
// writes anything, and refuses an archive of another format version. A bad
// entry is skipped with a warning, the other entries are imported, and the
// import fails once it is over. A dry run reports the same events and
// returns the same error, and writes nothing.
func (service *TaskService) ImportTasks(dto ImportTasksDto) error {
	if dto.ProjectSlug == "" {
		return errors.New("project slug is required")
	}
	if dto.Path == "" {
		return errors.New("archive path is required")
	}

	archive, err := service.repo.ReadTaskArchive(dto.Path)
	var broken *BrokenArchiveError
	if errors.As(err, &broken) {
		return fmt.Errorf("could not import %s: %w, %s", dto.Path, err, freshExportAdvice)
	}
	if err != nil {
		return fmt.Errorf("could not import %s: %w", dto.Path, err)
	}
	manifest := archive.Manifest
	if manifest.FormatVersion != ArchiveFormatVersion {
		return fmt.Errorf(
			"could not import %s: it has archive format version %d and drg reads version %d, %s",
			dto.Path, manifest.FormatVersion, ArchiveFormatVersion, freshExportAdvice,
		)
	}

	if dto.IsDryRun {
		service.progress.Report(ImportDryRunStarted{ProjectSlug: dto.ProjectSlug, Path: dto.Path})
	}
	if manifest.ProjectSlug != dto.ProjectSlug {
		service.progress.Report(ProjectSlugRewritten{ArchiveSlug: manifest.ProjectSlug, ProjectSlug: dto.ProjectSlug})
	}

	files, badEntries := checkArchivedTasks(archive)
	for _, badEntry := range badEntries {
		service.progress.Report(badEntry)
	}

	var storedByID map[TaskID]*Task
	if dto.IsDryRun {
		storedByID, err = service.storedTasksByID(dto.ProjectSlug)
		if err != nil {
			return fmt.Errorf("could not import %s: %w", dto.Path, err)
		}
	}

	summary := TasksImported{ProjectSlug: dto.ProjectSlug, SkippedAsBad: len(badEntries), IsDryRun: dto.IsDryRun}
	for _, file := range files {
		archived := file.Task
		archived.ProjectSlug = dto.ProjectSlug

		var isStored bool
		var skipReason ImportSkipReason
		decide := func(stored *Task) error {
			isStored = stored != nil
			skipReason = resolveImport(stored, archived)
			if skipReason != "" {
				return ErrTaskUnchanged
			}
			return nil
		}

		// A dry run decides against the tasks it listed. A write creates the
		// tasks directory and a lock file even when decide keeps the task.
		var isWritten bool
		if dto.IsDryRun {
			isWritten = decide(storedByID[archived.ID]) == nil
		} else {
			isWritten, err = service.repo.WriteTask(dto.ProjectSlug, archived, decide)
			if err != nil {
				return fmt.Errorf("could not import task %s: %w", archived.ID, err)
			}
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
	if len(badEntries) > 0 {
		return fmt.Errorf("skipped %s of %s as bad, a fresh export on the source device fixes them", formatEntryCount(len(badEntries)), dto.Path)
	}
	return nil
}

func (service *TaskService) storedTasksByID(projectSlug string) (map[TaskID]*Task, error) {
	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return nil, err
	}
	storedByID := make(map[TaskID]*Task, len(tasks))
	for _, stored := range tasks {
		storedByID[stored.ID] = stored
	}
	return storedByID, nil
}

// checkArchivedTasks splits the task files of an archive into the ones import
// can write and the bad ones. A bad entry the repository found comes first.
func checkArchivedTasks(archive TaskArchive) ([]TaskFile, []BadArchiveEntry) {
	badEntries := slices.Clone(archive.BadEntries)
	files := make([]TaskFile, 0, len(archive.Files))
	for _, file := range archive.Files {
		reason := checkArchivedTask(file.Task)
		if reason != "" {
			badEntries = append(badEntries, BadArchiveEntry{Name: file.Name, Reason: reason})
			continue
		}
		files = append(files, file)
	}
	return files, badEntries
}

// checkArchivedTask says what makes an archived task unfit to write, or
// returns an empty reason. The task id names the task file, so an id that is
// not a UUID could name a file outside the tasks directory.
func checkArchivedTask(archived *Task) string {
	if !common.IsUUID(string(archived.ID)) {
		return fmt.Sprintf("its id %q is not a UUID", archived.ID)
	}
	if !KnownStatus(archived.Status) {
		return fmt.Sprintf("it has unknown status %q, drg knows %s", archived.Status, FormatStatuses(Statuses))
	}
	return ""
}

func formatEntryCount(count int) string {
	if count == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", count)
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
