// Package app builds the configs, repositories, runners and services every
// interface of drudge works with.
package app

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/exec"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/filesystem"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/gitcli"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	remoteadapter "github.com/IgorBolotnikov/DRUDGE/internal/adapters/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/streams"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// Configs holds the local config of the project in the current directory and
// the global config.
type Configs struct {
	Local  *config.LocalConfig
	Global *config.GlobalConfig
}

// Services holds the services of the project in the current directory.
type Services struct {
	Tasks    *task.TaskService
	Drudgers *drudger.DrudgerService
}

// LoadConfigs loads the local and the global config. It fails when the
// current directory is not a project.
func LoadConfigs() (*Configs, error) {
	localCfg, err := config.LoadLocal()
	if err != nil {
		return nil, err
	}
	globalCfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &Configs{Local: localCfg, Global: globalCfg}, nil
}

// NewServices wires the task and the Drudger services of the project configs
// describe. Both report to progress.
func NewServices(configs *Configs, progress common.Progress) (*Services, error) {
	tasks := NewTaskService(configs.Local.ProjectSlug, configs.Local, configs.Global, progress)
	drudgers := persistence.NewFileDrudgerRepository("")
	runs := persistence.NewFileRunRepository("")
	cmdRunner := exec.NewCommandRunner()
	remoteSettings, err := config.ResolveRemote(configs.Local, configs.Global)
	if err != nil {
		return nil, err
	}
	settings, err := newDrudgerSettings(configs.Local, configs.Global, remoteSettings.PullRequests)
	if err != nil {
		return nil, err
	}
	pullRequestRemote, err := newPullRequestRemote(remoteSettings, cmdRunner)
	if err != nil {
		return nil, err
	}

	return &Services{
		Tasks:    tasks,
		Drudgers: drudger.New(progress, settings, tasks, drudgers, runs, cmdRunner, newGitOperations(configs.Global), pullRequestRemote, streamReaders()),
	}, nil
}

// NewTaskService wires a task service over the task files of a project.
// localCfg is nil outside a project directory. The default status then comes
// from the global config and a task may list no repositories.
func NewTaskService(projectSlug string, localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, progress common.Progress) *task.TaskService {
	if localCfg == nil {
		localCfg = &config.LocalConfig{ProjectSlug: projectSlug}
	}
	repo := persistence.NewFileTaskRepository(projectSlug)
	return task.NewTaskService(repo, progress, config.ResolveDefaultTaskStatus(localCfg, globalCfg), repositoryPaths(localCfg))
}

// NewProjectService wires a project service over the project records, the
// local config file, the git binary and the files of the project directory.
func NewProjectService(progress common.Progress) (*project.ProjectService, error) {
	globalCfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	home, err := common.HomeDir()
	if err != nil {
		return nil, err
	}
	linker := config.NewLocalConfigLinker(home)
	return project.NewProjectService(persistence.NewFileProjectRepository(""), linker, linker, newGitOperations(globalCfg), filesystem.NewRepositoryFiles(), config.NewGlobalSettingsReader(home), progress), nil
}

// streamReaders maps every harness drudge knows how to read the event stream
// of onto its reader.
func streamReaders() map[drudger.Harness]drudger.StreamReader {
	return map[drudger.Harness]drudger.StreamReader{
		drudger.HarnessClaudeCode: streams.NewClaudeCodeReader(),
		drudger.HarnessOpencode:   streams.NewOpencodeReader(),
	}
}

// repositoryPaths lists the paths of the repositories the local config
// records.
func repositoryPaths(localCfg *config.LocalConfig) []string {
	paths := make([]string, 0, len(localCfg.Repositories))
	for _, repository := range localCfg.Repositories {
		paths = append(paths, repository.Path)
	}
	return paths
}

// newDrudgerSettings picks what the Drudger service reads out of the configs.
func newDrudgerSettings(localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, pullRequests config.PullRequestSettings) (drudger.Settings, error) {
	promptPath, err := config.ResolvePromptPath(localCfg, globalCfg)
	if err != nil {
		return drudger.Settings{}, err
	}
	timeouts := globalCfg.Drudger.SandboxTimeouts
	return drudger.Settings{
		ProjectSlug:           localCfg.ProjectSlug,
		Repositories:          localCfg.Repositories,
		Env:                   globalCfg.Drudger.Env,
		Harness:               globalCfg.Drudger.Harness,
		MaxConcurrentDrudgers: config.ResolveMaxConcurrentDrudgers(localCfg, globalCfg),
		PromptPath:            promptPath,
		Model:                 globalCfg.Drudger.Model,
		EnvNames:              localCfg.Drudger.Env,
		BranchFormat:          config.ResolveBranchFormat(localCfg, globalCfg),
		SandboxTimeouts: drudger.SandboxTimeouts{
			List:        timeouts.List(),
			Create:      timeouts.Create(),
			Remove:      timeouts.Remove(),
			Setup:       timeouts.Setup(),
			FirstOutput: timeouts.FirstOutput(),
		},
		PullRequests: drudger.PullRequestSettings{
			IsDraft:      pullRequests.IsDraft,
			TitleFormat:  pullRequests.TitleFormat,
			TemplatePath: pullRequests.TemplatePath,
			StepsPath:    pullRequests.StepsPath,
		},
	}, nil
}

// newPullRequestRemote wires the remote of the configured provider, and
// returns nil when pull requests are off.
func newPullRequestRemote(settings config.RemoteSettings, runner *exec.CommandRunner) (remote.Remote, error) {
	if !settings.PullRequests.IsEnabled {
		return nil, nil
	}
	return remoteadapter.New(settings.Provider, runner, settings.Timeout)
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
