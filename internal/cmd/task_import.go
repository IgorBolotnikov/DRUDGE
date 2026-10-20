package cmd

import (
	"flag"

	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const archiveFileArg = "file"

// taskImportFlags holds the flags of drg task import.
type taskImportFlags struct {
	project string
}

func (flags *taskImportFlags) declare(fs *flag.FlagSet) {
	fs.StringVar(&flags.project, projectFlagName, "", "The `slug` of the project to import into, the project of the current directory when left out")
	alias(fs, projectFlagShortName, projectFlagName)
}

// taskImport adds the tasks of a zip archive the project does not have yet.
func taskImport(flags *taskImportFlags, path string) error {
	out := newCommandPrinter()
	projects, err := newProjectService(out)
	if err != nil {
		return err
	}
	importingProject, err := projects.ResolveProject(flags.project)
	if err != nil {
		return err
	}

	globalCfg, err := config.Load()
	if err != nil {
		return err
	}
	tasks := newTaskService(importingProject.Slug, nil, globalCfg, out)

	return tasks.ImportTasks(task.ImportTasksDto{ProjectSlug: importingProject.Slug, Path: path})
}
