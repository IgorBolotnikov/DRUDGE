package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// tasksTabTitle is the title of the Tab with the task list.
const tasksTabTitle = "Tasks"

// barHeight is how many lines the top bar and the bottom bar take together.
const barHeight = 2

// warningPrefix starts a warning in the bottom bar.
const warningPrefix = "! "

// warningSeparator joins the warnings of the bottom bar.
const warningSeparator = "  "

// The task list loads every task of the project on one page.
const (
	firstPage    = 1
	allOnOnePage = 0
)

var quitKey = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit"))

// Model is the root of the screen: a top bar, one Tab and a bottom bar. It
// loads the data and sends it to the Views.
type Model struct {
	projectSlug string
	source      TaskSource
	styles      *Styles
	warnings    []string
	tab         *Tab
	help        help.Model
	width       int
	height      int
}

// NewModel returns the screen of the project projectSlug. It lists the tasks
// of source and shows warnings in the bottom bar.
func NewModel(projectSlug string, source TaskSource, styles *Styles, warnings []string) *Model {
	keyHelp := help.New()
	keyHelp.Styles = styles.Help
	return &Model{
		projectSlug: projectSlug,
		source:      source,
		styles:      styles,
		warnings:    warnings,
		tab:         NewTab(tasksTabTitle, NewTile(NewTaskListView(styles), styles)),
		help:        keyHelp,
	}
}

// Init loads the tasks of the project.
func (model *Model) Init() tea.Cmd {
	return model.loadTasks
}

func (model *Model) loadTasks() tea.Msg {
	listed, err := model.source.ListTasks(model.projectSlug, task.ListTasksFilter{}, firstPage, allOnOnePage)
	return tasksLoadedMsg{tasks: listed.Items, err: err}
}

func (model *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height
		model.help.SetWidth(msg.Width)
		model.tab.SetSize(msg.Width, max(msg.Height-barHeight, 0))
		return model, nil
	case tea.KeyPressMsg:
		if key.Matches(msg, quitKey) {
			return model, tea.Quit
		}
		return model, model.tab.Focused().view.Update(msg)
	case tasksLoadedMsg:
		return model, model.tab.Broadcast(msg)
	}
	return model, nil
}

func (model *Model) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true
	if model.width <= 0 || model.height <= 0 {
		return view
	}
	lines := []string{model.topBar()}
	if model.height > barHeight {
		lines = append(lines, model.tab.Render())
	}
	if model.height > 1 {
		lines = append(lines, model.bottomBar())
	}
	view.Content = strings.Join(lines, "\n")
	return view
}

// topBar shows the title of the Tab on the left and the project on the right.
func (model *Model) topBar() string {
	return spread(model.styles.ActiveTab.Render(" "+model.tab.Title+" "), model.styles.Project.Render(model.projectSlug), model.width)
}

// bottomBar shows the warnings on the left and the key hints on the right.
func (model *Model) bottomBar() string {
	bindings := append(model.tab.Focused().view.KeyHints(), quitKey)
	hints := model.help.ShortHelpView(bindings)
	warnings := make([]string, 0, len(model.warnings))
	for _, warning := range model.warnings {
		warnings = append(warnings, model.styles.Warning.Render(warningPrefix+warning))
	}
	return spread(strings.Join(warnings, warningSeparator), hints, model.width)
}

// spread puts left at the start and right at the end of a line width cells
// wide. It cuts left to keep right whole and cuts right when it is wider than
// the line.
func spread(left, right string, width int) string {
	right = ansi.Truncate(right, width, "")
	room := width - ansi.StringWidth(right)
	if room <= 1 {
		return strings.Repeat(" ", room) + right
	}
	left = ansi.Truncate(left, room-1, ellipsis)
	return fit(left, room) + right
}
