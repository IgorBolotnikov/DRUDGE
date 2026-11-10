// Package tui is the fullscreen terminal interface of drudge.
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/IgorBolotnikov/DRUDGE/internal/app"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
)

// TaskSource lists the tasks of a project.
type TaskSource interface {
	ListTasks(projectSlug string, filter task.ListTasksFilter, page int, size int) (common.Page[task.ListedTask], error)
}

// silentProgress drops every event a service reports.
type silentProgress struct{}

func (silentProgress) Report(any) {}

// Run shows the project of the current directory full screen until the user
// quits. It fails before it takes over the terminal when the TUI config file
// is invalid or the current directory is not a project.
func Run() error {
	palette, warnings := loadTheme()
	screenStyles := styles.New(palette)
	tabs, err := loadTabs(screenStyles)
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
	tasks := app.NewTaskService(configs.Local.ProjectSlug, configs.Local, configs.Global, silentProgress{})

	var options []tea.ProgramOption
	if !palette.IsColorOn(theme.Stdout) {
		options = append(options, tea.WithColorProfile(colorprofile.ASCII))
	}
	model, err := NewModel(shownProject, tasks, screenStyles, tabs, warnings)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(model, options...).Run()
	return err
}

// loadTabs builds the Tabs of the TUI config file. An error starts with the
// path of the file.
func loadTabs(screenStyles *styles.Styles) ([]*Tab, error) {
	home, err := common.HomeDir()
	if err != nil {
		return nil, err
	}
	path := common.TuiConfigPath(home)
	config, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	tabs, err := BuildTabs(config, screenStyles)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return tabs, nil
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
