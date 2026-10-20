package cmd

import (
	"flag"

	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

// Flags that name the project a command works on.
const (
	projectFlagName      = "project"
	projectFlagShortName = "p"
)

// Flags that name the file a command writes.
const (
	outputFlagName      = "output"
	outputFlagShortName = "o"
)

// taskExportFlags holds the flags of drg task export.
type taskExportFlags struct {
	project string
	output  string
}

func (flags *taskExportFlags) declare(fs *flag.FlagSet) {
	fs.StringVar(&flags.project, projectFlagName, "", "The `slug` of the project to export, the project of the current directory when left out")
	alias(fs, projectFlagShortName, projectFlagName)
	fs.StringVar(&flags.output, outputFlagName, "", "Write the archive to `path`, <slug>-<count>-tasks-<YYYYMMDD-HHMMSS>.zip in the current directory when left out")
	alias(fs, outputFlagShortName, outputFlagName)
}

// taskExport writes every task of a project into a zip archive.
func taskExport(flags *taskExportFlags) error {
	out := newCommandPrinter()
	projects, err := newProjectService(out)
	if err != nil {
		return err
	}
	exportedProject, err := projects.ResolveProject(flags.project)
	if err != nil {
		return err
	}

	globalCfg, err := config.Load()
	if err != nil {
		return err
	}
	tasks := newTaskService(exportedProject.Slug, nil, globalCfg, out)

	exported, err := tasks.ExportTasks(task.ExportTasksDto{ProjectSlug: exportedProject.Slug, Path: flags.output})
	if err != nil {
		return err
	}

	path := out.Theme().Paint(theme.Stdout, theme.RoleMuted, exported.Path)
	out.Result("Exported %s of project %s to %s", task.FormatTaskCount(exported.Count), exportedProject.Slug, path)
	return nil
}
