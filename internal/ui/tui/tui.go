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
// quits. It fails before it takes over the terminal when the current
// directory is not a project.
func Run() error {
	configs, err := app.LoadConfigs()
	if err != nil {
		return err
	}
	palette, warnings := loadTheme()
	tasks := app.NewTaskService(configs.Local.ProjectSlug, configs.Local, configs.Global, silentProgress{})

	var options []tea.ProgramOption
	if !palette.IsColorOn(theme.Stdout) {
		options = append(options, tea.WithColorProfile(colorprofile.ASCII))
	}
	screenStyles := styles.New(palette)
	tabs, err := defaultTabs(screenStyles)
	if err != nil {
		return err
	}
	model, err := NewModel(configs.Local.ProjectSlug, tasks, screenStyles, tabs, warnings)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(model, options...).Run()
	return err
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
