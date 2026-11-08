package cli

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/app"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/cli/printer"
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
	configs, err := app.LoadConfigs()
	if err != nil {
		return nil, err
	}

	out := newCommandPrinter()
	services, err := app.NewServices(configs, newCLIProgress(out))
	if err != nil {
		return nil, err
	}

	return &commandDeps{
		localCfg:  configs.Local,
		globalCfg: configs.Global,
		out:       out,
		tasks:     services.Tasks,
		drudger:   services.Drudgers,
	}, nil
}
