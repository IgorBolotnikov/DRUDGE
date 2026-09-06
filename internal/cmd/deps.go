package cmd

import (
	"drudge/internal/adapters/exec"
	"drudge/internal/adapters/persistence"
	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/drudger"
	"drudge/internal/task"
)

// commandDeps holds the dependency graph a subcommand works with. It is built
// around the project the current directory is linked to.
type commandDeps struct {
	localCfg *config.LocalConfig
	log      *common.Logger
	tasks    *task.TaskService
	drudger  *drudger.DrudgerService
}

// newCommandDeps wires up what a subcommand needs to reach a project's tasks
// and the Drudgers working on them.
func newCommandDeps() (*commandDeps, error) {
	localCfg, err := config.LoadLocal()
	if err != nil {
		return nil, err
	}

	globalCfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	log := common.NewLogger("")
	repo := persistence.NewFileTaskRepository(localCfg.ProjectSlug)
	tasks := task.NewTaskService(repo, log)
	drudgers := persistence.NewFileDrudgerRepository("")
	cmdRunner := exec.NewCommandRunner()

	return &commandDeps{
		localCfg: localCfg,
		log:      log,
		tasks:    tasks,
		drudger:  drudger.New(log, localCfg, globalCfg, tasks, drudgers, cmdRunner),
	}, nil
}
