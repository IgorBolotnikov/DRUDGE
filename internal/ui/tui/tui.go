// Package tui is the fullscreen terminal interface of drudge.
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/IgorBolotnikov/DRUDGE/internal/app"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
)

// silentProgress drops every event a service reports.
type silentProgress struct{}

func (silentProgress) Report(any) {}

// Run shows the project of the current directory full screen until the user
// quits. It fails before it takes over the terminal when the TUI config file
// is invalid or the current directory is not a project.
func Run() error {
	palette, warnings := loadTheme()
	screenStyles := styles.New(palette)
	config, tabs, err := loadConfig(screenStyles)
	if err != nil {
		return err
	}
	configs, err := app.LoadConfigs()
	if err != nil {
		return err
	}
	projects, err := app.NewProjectService(silentProgress{})
	if err != nil {
		return err
	}
	shownProject, err := projects.LookupProject(configs.Local.ProjectSlug)
	if err != nil {
		return err
	}
	services, err := app.NewServices(configs, silentProgress{})
	if err != nil {
		return err
	}

	var options []tea.ProgramOption
	if !palette.IsColorOn(theme.Stdout) {
		options = append(options, tea.WithColorProfile(colorprofile.ASCII))
	}
	sources := feeds.Sources{Tasks: services.Tasks, Drudgers: services.Drudgers}
	model, err := NewModel(shownProject, sources, screenStyles, tabs, config.Intervals(), config.IsMouseCaptured(), warnings)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(model, options...).Run()
	return err
}

// loadConfig loads the TUI config file and builds its Tabs. An error starts
// with the path of the file.
func loadConfig(screenStyles *styles.Styles) (Config, []*Tab, error) {
	home, err := common.HomeDir()
	if err != nil {
		return Config{}, nil, err
	}
	path := common.TuiConfigPath(home)
	config, err := LoadConfig(path)
	if err != nil {
		return Config{}, nil, err
	}
	tabs, err := BuildTabs(config, screenStyles)
	if err != nil {
		return Config{}, nil, fmt.Errorf("%s: %w", path, err)
	}
	return config, tabs, nil
}

// loadTheme loads the theme and returns the warnings about it. A theme that
// fails to load leaves the default theme.
func loadTheme() (*theme.Theme, []string) {
	var warnings []string
	palette, err := theme.Load("")
	if err != nil {
		palette = theme.NewTheme(theme.DefaultTheme())
		warnings = append(warnings, fmt.Sprintf("cannot load the theme, using the default one: %v", err))
	}
	for _, skipped := range palette.SkippedOverrides() {
		warnings = append(warnings, fmt.Sprintf("theme.json: %q is not a color for role %s, using the theme's own", skipped.Value, skipped.Role))
	}
	return palette, warnings
}
