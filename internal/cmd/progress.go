package cmd

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// cliProgress renders the events a domain service reports as the lines drg
// prints today. A later change can give these lines a different layout and
// colors without touching a service.
type cliProgress struct {
	log *common.Logger
}

// newCLIProgress builds a progress port that prints through log.
func newCLIProgress(log *common.Logger) *cliProgress {
	return &cliProgress{log: log}
}

func (p *cliProgress) Report(event any) {
	switch event := event.(type) {
	case task.TaskCreated:
		p.log.Info("Created task [%s] %s", event.Task.ID, event.Task.Title)
	case task.TaskRemovalDeclined:
		p.log.Info("Left task [%s] %s alone", event.Task.ID, event.Task.Title)
	case task.TaskRemoved:
		p.log.Info("Removed task [%s] %s", event.Task.ID, event.Task.Title)
		if event.HasRun {
			p.log.Info("Its run directory went with it")
		}
	case task.TasksUnblocked:
		p.log.Info("Took it off the blockers of %s", task.FormatTaskCount(event.Count))
	case task.TasksUngrouped:
		p.log.Info("Ungrouped %s that belonged to it", task.FormatTaskCount(event.Count))
	case task.TaskMarkedDone:
		p.log.Info("Task [%s] %s is %q", event.Task.ID, event.Task.Title, event.Task.Status)
	case task.TaskEdited:
		p.log.Info("Updated task [%s] %s, it is now %q", event.Task.ID, event.Task.Title, event.Task.Status)
	case project.ProjectCreated:
		p.log.Info("Created project %s", event.Project.Name)
	case project.ProjectRenamed:
		p.log.Info("Renamed project %s from %q to %q", event.Slug, event.OldName, event.NewName)
	case release.DownloadStarted:
		p.log.Info("Downloading %s (%s)", event.ArchiveName, event.Version)
	}
}
