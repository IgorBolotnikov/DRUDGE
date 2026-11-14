// Package report turns the progress events of the domain services into the
// words every interface of drudge shows for them. A line carries its kind and
// its level. The interface picks the glyphs, the indents and the colors.
package report

import (
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// Level is how a line went.
type Level string

const (
	// LevelInfo is a line that reports no outcome, like a header or a field.
	LevelInfo    Level = "info"
	LevelStep    Level = "step"
	LevelSuccess Level = "success"
	LevelSkipped Level = "skipped"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
)

// Kind is the place a line takes in the layout of a command.
type Kind string

const (
	// KindLine is a line about the work. It sits inside the open group.
	KindLine Kind = "line"
	// KindHeader opens a group.
	KindHeader Kind = "header"
	// KindResult closes a group with its outcome.
	KindResult Kind = "result"
	// KindDetail is subprocess output, one level deeper than the lines
	// around it.
	KindDetail Kind = "detail"
	// KindField is a labelled value. A field with an empty label continues
	// the field before it.
	KindField Kind = "field"
	// KindBlock is text shown as it is, like a prompt. It may span several
	// lines.
	KindBlock Kind = "block"
)

// Line is one line of what an event says. The text of a step ends without an
// ellipsis.
type Line struct {
	Kind  Kind
	Level Level
	// Label names the value of a field. Other kinds leave it empty.
	Label string
	Text  string
}

// TaskNamer names a task inside the text of a line. An interface passes the
// one that styles the name for its screen.
type TaskNamer func(named *task.Task) string

const (
	unblockedLabel = "Unblocked"
	adviceLabel    = "Advice"
)

// Lines returns the lines an event says, in the order they show. The first
// line stands for the event on its own. An event that shows nothing returns
// no lines.
func Lines(event any, nameTask TaskNamer) []Line {
	switch event := event.(type) {
	case task.TaskCreated:
		return []Line{done("Created task %s", nameTask(event.Task))}
	case task.TaskRemovalDeclined:
		return []Line{skip("Left task %s alone", nameTask(event.Task))}
	case task.TaskRemovalStarted:
		return []Line{header("Removing task %s", nameTask(event.Task))}
	case task.RunDirectoryRemoved:
		return []Line{done("Removed its run directory")}
	case task.TaskRemoved:
		return []Line{result(LevelSuccess, "Removed task %s", nameTask(event.Task))}
	case task.TasksUnblocked:
		return []Line{done("Took it off the blockers of %s", task.FormatTaskCount(event.Count))}
	case task.TasksUngrouped:
		return []Line{done("Ungrouped %s that belonged to it", task.FormatTaskCount(event.Count))}
	case task.BranchesCleanupFailed:
		return []Line{warn("Task %s is removed, but the branches it left could not be cleaned up: %v", event.TaskID, event.Err)}
	case task.TaskUnlinkFailed:
		return []Line{taskUnlinkFailed(event)}
	case task.TaskMarkedDone:
		return []Line{done("Task %s is %q", nameTask(event.Task), event.Task.Status)}
	case task.TaskEdited:
		return []Line{done("Updated task %s, it is now %q", nameTask(event.Task), event.Task.Status)}
	case task.ImportDryRunStarted:
		return []Line{header("Dry run of an import of %s into project %s", event.Path, event.ProjectSlug)}
	case task.ProjectSlugRewritten:
		return []Line{warn("The archive holds tasks of project %s, they go into project %s", event.ArchiveSlug, event.ProjectSlug)}
	case task.BadArchiveEntry:
		return []Line{warn("Skipped %s, %s", event.Name, event.Reason)}
	case task.TaskImported:
		return []Line{taskImported(event, nameTask)}
	case task.TaskImportUpdated:
		return []Line{done("Updated task %s, the archive holds a newer copy", nameTask(event.Task))}
	case task.TaskImportSkipped:
		return []Line{taskImportSkipped(event, nameTask)}
	case task.TasksImported:
		return []Line{tasksImported(event)}
	case project.ProjectCreated:
		// Project init prints the result line itself, after the warnings
		// about its repositories.
		return nil
	case project.ProjectRenamed:
		return []Line{done("Renamed project %s from %q to %q", event.Slug, event.OldName, event.NewName)}
	case project.ProjectRemoved:
		return []Line{done("Removed project %s", event.Name)}
	case project.ProjectAlreadyGone:
		return []Line{skip("Project %s was already gone", event.Slug)}
	case release.DownloadStarted:
		return []Line{step("Downloading %s (%s)", event.ArchiveName, event.Version)}
	default:
		return drudgerLines(event, nameTask)
	}
}

func taskImported(event task.TaskImported, nameTask TaskNamer) Line {
	if event.Task.Status == task.StatusInProgress {
		return done("Added task %s, it is in progress on the other device", nameTask(event.Task))
	}
	return done("Added task %s", nameTask(event.Task))
}

func taskImportSkipped(event task.TaskImportSkipped, nameTask TaskNamer) Line {
	if event.Reason == task.SkipReasonInProgress {
		return skip("Skipped task %s, it is in progress here", nameTask(event.Task))
	}
	return skip("Skipped task %s, the archive holds no newer copy", nameTask(event.Task))
}

// tasksImported closes an import with how many tasks it added, updated and
// skipped, or would have in a dry run.
func tasksImported(event task.TasksImported) Line {
	counts := fmt.Sprintf(
		"added %d, updated %d, skipped %d as older, %d as in progress and %d as bad",
		event.Added, event.Updated, event.SkippedAsOlder, event.SkippedAsInProgress, event.SkippedAsBad,
	)
	if event.IsDryRun {
		return result(LevelSkipped, "Wrote nothing, it was a dry run. It would have %s", counts)
	}
	if event.Added == 0 && event.Updated == 0 {
		return result(LevelSkipped, "Changed no tasks in project %s, %s", event.ProjectSlug, counts)
	}
	return result(LevelSuccess, "Imported into project %s, %s", event.ProjectSlug, counts)
}

func taskUnlinkFailed(event task.TaskUnlinkFailed) Line {
	if event.IsHeld {
		return warn("Task %s is removed, but another drudge command is working on task %s, which still names it", event.RemovedID, event.LinkedID)
	}
	return warn("Task %s is removed, but task %s still names it: %v", event.RemovedID, event.LinkedID, event.Err)
}

func header(format string, args ...any) Line {
	return Line{Kind: KindHeader, Level: LevelInfo, Text: fmt.Sprintf(format, args...)}
}

func result(level Level, format string, args ...any) Line {
	return Line{Kind: KindResult, Level: level, Text: fmt.Sprintf(format, args...)}
}

func done(format string, args ...any) Line {
	return Line{Kind: KindLine, Level: LevelSuccess, Text: fmt.Sprintf(format, args...)}
}

func step(format string, args ...any) Line {
	return Line{Kind: KindLine, Level: LevelStep, Text: fmt.Sprintf(format, args...)}
}

func skip(format string, args ...any) Line {
	return Line{Kind: KindLine, Level: LevelSkipped, Text: fmt.Sprintf(format, args...)}
}

func warn(format string, args ...any) Line {
	return Line{Kind: KindLine, Level: LevelWarning, Text: fmt.Sprintf(format, args...)}
}

func failed(format string, args ...any) Line {
	return Line{Kind: KindLine, Level: LevelError, Text: fmt.Sprintf(format, args...)}
}

func detail(format string, args ...any) Line {
	return Line{Kind: KindDetail, Level: LevelInfo, Text: fmt.Sprintf(format, args...)}
}

func field(label string, value string) Line {
	return Line{Kind: KindField, Level: LevelInfo, Label: label, Text: value}
}

func block(text string) Line {
	return Line{Kind: KindBlock, Level: LevelInfo, Text: text}
}
