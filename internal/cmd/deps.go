package cmd

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/exec"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/gitcli"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

type commandDeps struct {
	localCfg *config.LocalConfig
	log      *common.Logger
	tasks    *task.TaskService
	drudger  *drudger.DrudgerService
}

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
		drudger:  drudger.New(log, localCfg, globalCfg, tasks, drudgers, cmdRunner, newGitOperations(globalCfg)),
	}, nil
}

// newProjectService wires a project service over the project records and the
// git binary.
func newProjectService(log *common.Logger) (*project.ProjectService, error) {
	globalCfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return project.NewProjectService(persistence.NewFileProjectRepository(""), newGitOperations(globalCfg), log), nil
}

// newGitOperations wires the git adapter with the configured timeouts.
func newGitOperations(globalCfg *config.GlobalConfig) git.Operations {
	timeouts := globalCfg.Drudger.GitTimeouts
	return gitcli.New(exec.NewCommandRunner(), git.Timeouts{
		Fetch:    timeouts.Fetch(),
		Worktree: timeouts.Worktree(),
		Command:  timeouts.Command(),
	})
}
