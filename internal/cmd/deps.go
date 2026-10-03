package cmd

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/exec"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/gitcli"
	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	remoteadapter "github.com/IgorBolotnikov/DRUDGE/internal/adapters/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

type commandDeps struct {
	localCfg  *config.LocalConfig
	globalCfg *config.GlobalConfig
	out       *printer.Printer
	tasks     *task.TaskService
	drudger   *drudger.DrudgerService
}

// NewLogger builds the logger of a command. Its errors and warnings carry the
// labels of the loaded theme.
func NewLogger() *common.Logger {
	return newThemedLogger(theme.LoadOrDefault())
}

// newThemedLogger builds the logger of a command whose labels come from
// palette.
func newThemedLogger(palette *theme.Theme) *common.Logger {
	return common.NewLogger("", common.Labels{Error: palette.ErrorLabel(), Warn: palette.WarnLabel()})
}

// newCommandPrinter builds the printer of a command over the loaded theme. A
// theme that fails to load leaves the default theme. The printer warns about
// that and about every override the theme skipped.
func newCommandPrinter() *printer.Printer {
	palette, err := theme.Load("")
	if err != nil {
		palette = theme.NewTheme(theme.DefaultTheme())
	}
	out := printer.NewPrinter(newThemedLogger(palette), palette)
	if err != nil {
		out.Warn("cannot load the theme, using the default one: %v", err)
	}
	for _, skipped := range palette.SkippedOverrides() {
		out.Warn("theme.json: %q is not a color for role %s, using the theme's own", skipped.Value, skipped.Role)
	}
	return out
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

	out := newCommandPrinter()
	progress := newCLIProgress(out)
	repo := persistence.NewFileTaskRepository(localCfg.ProjectSlug)
	tasks := task.NewTaskService(repo, progress, config.ResolveDefaultTaskStatus(localCfg, globalCfg))
	drudgers := persistence.NewFileDrudgerRepository("")
	runs := persistence.NewFileRunRepository("")
	cmdRunner := exec.NewCommandRunner()
	remoteSettings, err := config.ResolveRemote(localCfg, globalCfg)
	if err != nil {
		return nil, err
	}
	settings, err := newDrudgerSettings(localCfg, globalCfg, remoteSettings.PullRequests)
	if err != nil {
		return nil, err
	}
	pullRequestRemote, err := newPullRequestRemote(remoteSettings, cmdRunner)
	if err != nil {
		return nil, err
	}

	return &commandDeps{
		localCfg:  localCfg,
		globalCfg: globalCfg,
		out:       out,
		tasks:     tasks,
		drudger:   drudger.New(progress, settings, tasks, drudgers, runs, cmdRunner, newGitOperations(globalCfg), pullRequestRemote),
	}, nil
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
		SandboxTimeouts: drudger.SandboxTimeouts{
			List:   timeouts.List(),
			Create: timeouts.Create(),
			Remove: timeouts.Remove(),
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

// newProjectService wires a project service over the project records, the
// local config file and the git binary.
func newProjectService(out *printer.Printer) (*project.ProjectService, error) {
	globalCfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	home, err := common.HomeDir()
	if err != nil {
		return nil, err
	}
	linker := config.NewLocalConfigLinker(home)
	return project.NewProjectService(persistence.NewFileProjectRepository(""), linker, newGitOperations(globalCfg), newCLIProgress(out)), nil
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
