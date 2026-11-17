package cli

import (
	"flag"

	"github.com/IgorBolotnikov/DRUDGE/internal/app"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const archiveFileArg = "file"

// taskImportFlags holds the flags of drg task import.
type taskImportFlags struct {
	project  string
	isDryRun bool
}

func (flags *taskImportFlags) declare(fs *flag.FlagSet) {
	fs.StringVar(&flags.project, projectFlagName, "", "The `slug` of the project to import into, the project of the current directory when left out")
	alias(fs, projectFlagShortName, projectFlagName)
	fs.BoolVar(&flags.isDryRun, dryRunFlagName, false, "Print what the import would do and write nothing")
}

// taskImport adds the tasks of a zip archive the project does not have yet.
func taskImport(flags *taskImportFlags, path string) error {
	progress := newCLIProgress(newCommandPrinter())
	projects, err := app.NewProjectService(progress)
	if err != nil {
		return err
	}
	importingProject, err := projects.ResolveProject(flags.project)
	if err != nil {
		return err
	}

	configs, err := app.LoadGlobalConfig()
	if err != nil {
		return err
	}
	tasks := app.NewTaskService(importingProject.Slug, configs, progress)

	return tasks.ImportTasks(task.ImportTasksDto{ProjectSlug: importingProject.Slug, Path: path, IsDryRun: flags.isDryRun})
}
