// Package taskdetail is the View that shows one task: its fields and its
// description rendered as markdown.
package taskdetail

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	glamouransi "charm.land/glamour/v2/ansi"
	glamourstyles "charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Name is the kind of the View that shows one task.
const Name = "task-detail"

// Description says in one line what the View shows.
const Description = "Shows the fields and the description of the task selected in a task list or a task board of the same tab."

// minDescriptionLines is the fewest description lines the View keeps below
// the fields. A View that cannot keep them folds the fields into one line.
const minDescriptionLines = 3

// descriptionGapLines is how many blank lines sit between the fields and the
// description. They stay in place while the description scrolls.
const descriptionGapLines = 1

// fieldTimeLayout is how the View shows a time field.
const fieldTimeLayout = "2006-01-02 15:04"

// Separators of the fields: between a label and its value, between the
// values of a list field and between the fields folded into one line.
const (
	labelGap        = "  "
	listSeparator   = ", "
	foldedSeparator = " · "
)

// Status segment of the View: the short ID of the task and how far the
// description is scrolled.
const scrolledStatusFormat = "%s %d%%"

// Labels of the fields of a task.
const (
	titleLabel        = "Title"
	idLabel           = "ID"
	statusLabel       = "Status"
	ticketLabel       = "Ticket"
	parentLabel       = "Parent"
	blockedByLabel    = "Blocked by"
	repositoriesLabel = "Repositories"
	pullRequestsLabel = "Pull requests"
	createdLabel      = "Created"
	updatedLabel      = "Updated"
	startedLabel      = "Started"
	finishedLabel     = "Finished"
)

// fieldLabels holds the label of every field a task may have. The label
// column takes the width of the widest of them, so it keeps its width from
// task to task.
var fieldLabels = []string{
	titleLabel, idLabel, statusLabel, ticketLabel, parentLabel, blockedByLabel,
	repositoriesLabel, pullRequestsLabel, createdLabel, updatedLabel, startedLabel, finishedLabel,
}

// Keys of the task detail. The scroll up key and the top key carry the help of
// their pair.
var (
	scrollUpKey     = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "scroll"))
	scrollDownKey   = key.NewBinding(key.WithKeys("j", "down"))
	halfPageUpKey   = key.NewBinding(key.WithKeys("u", "pgup"), key.WithHelp("u/d", "half page up/down"))
	halfPageDownKey = key.NewBinding(key.WithKeys("d", "pgdown"))
	scrollTopKey    = key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g/G", "top/bottom"))
	scrollBottomKey = key.NewBinding(key.WithKeys("G", "end"))
	openRunLogKey   = key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "runlog"))
)

// scrollLineCount is how many lines the scroll keys and one turn of the wheel
// move the description.
const scrollLineCount = 1

// Keys are the keys of the task detail the help lists.
var Keys = []key.Binding{scrollUpKey, halfPageUpKey, scrollTopKey, openRunLogKey}

// field is one labelled value of a task.
type field struct {
	label string
	value string
}

// View shows the task it was last handed, from the tasks of the last load
// that worked. The fields stay on top and the description scrolls below them.
type View struct {
	styles      *styles.Styles
	tasks       map[task.TaskID]*task.Task
	hasTasks    bool
	taskID      task.TaskID
	description viewport.Model
	// The task, the description and the width the description was last
	// rendered for. The View renders it again only when one of them changed.
	renderedTaskID      task.TaskID
	renderedDescription string
	renderedWidth       int
	width               int
	height              int
}

// New returns a task detail that shows a placeholder until it is handed a
// task.
func New(viewStyles *styles.Styles) *View {
	return &View{styles: viewStyles, tasks: map[task.TaskID]*task.Task{}, description: viewport.New()}
}

func (view *View) Name() string {
	return Name
}

// TakesAllKeys reports false. The View has no input that takes keys.
func (view *View) TakesAllKeys() bool {
	return false
}

// Status returns the short ID of the shown task and how far its description is
// scrolled, like 3f9a1c2e 40%. A description that fits leaves out how far. It
// is empty while no task is shown.
func (view *View) Status() string {
	shown, ok := view.shownTask()
	if !ok {
		return ""
	}
	shortID := task.ShortID(shown.ID)
	if view.description.TotalLineCount() <= view.description.Height() {
		return shortID
	}
	percent := int(math.Round(view.description.ScrollPercent() * 100))
	return fmt.Sprintf(scrolledStatusFormat, shortID, percent)
}

// ShowTask shows the task with id from the top of its description.
func (view *View) ShowTask(id task.TaskID) {
	view.taskID = id
	view.renderedTaskID = ""
	view.layOut()
}

// Open shows the task of an open task message from the top of its
// description. It refuses every other open message.
func (view *View) Open(msg views.OpenMsg) (isShown bool) {
	openTask, ok := msg.(views.OpenTaskMsg)
	if !ok {
		return false
	}
	view.ShowTask(openTask.TaskID)
	return true
}

// Feeds returns the tasks feed and the Sessions feed. The View ignores the
// messages of the Sessions feed, which records the outcome of each finished
// Session on its task for the tasks feed to show.
func (view *View) Feeds() []views.Feed {
	return []views.Feed{{Name: views.TasksFeed}, {Name: views.SessionsFeed}}
}

func (view *View) SetSize(width, height int) {
	view.width = width
	view.height = height
	view.layOut()
}

// Update takes the loaded tasks and scrolls the description on keys and on the
// wheel. L opens the runlog of the shown task.
func (view *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case views.TasksLoadedMsg:
		if msg.Err == nil {
			view.tasks = make(map[task.TaskID]*task.Task, len(msg.Tasks))
			for _, entry := range msg.Tasks {
				view.tasks[entry.Task.ID] = entry.Task
			}
			view.hasTasks = true
			view.layOut()
		}
	case tea.KeyPressMsg:
		if key.Matches(msg, openRunLogKey) {
			return view.openRunLog()
		}
		view.scroll(msg)
	case tea.MouseWheelMsg:
		view.handleWheel(msg)
	}
	return nil
}

// openRunLog asks to open the runlog of the shown task. It does nothing while
// no task is handed.
func (view *View) openRunLog() tea.Cmd {
	taskID := view.taskID
	if taskID == "" {
		return nil
	}
	return func() tea.Msg { return views.OpenRunLogMsg{TaskID: taskID} }
}

func (view *View) handleWheel(msg tea.MouseWheelMsg) {
	switch msg.Button {
	case tea.MouseWheelUp:
		view.description.ScrollUp(scrollLineCount)
	case tea.MouseWheelDown:
		view.description.ScrollDown(scrollLineCount)
	}
}

func (view *View) scroll(msg tea.KeyPressMsg) {
	switch {
	case key.Matches(msg, scrollUpKey):
		view.description.ScrollUp(scrollLineCount)
	case key.Matches(msg, scrollDownKey):
		view.description.ScrollDown(scrollLineCount)
	case key.Matches(msg, halfPageUpKey):
		view.description.HalfPageUp()
	case key.Matches(msg, halfPageDownKey):
		view.description.HalfPageDown()
	case key.Matches(msg, scrollTopKey):
		view.description.GotoTop()
	case key.Matches(msg, scrollBottomKey):
		view.description.GotoBottom()
	}
}

// shownTask returns the task the View shows. ok is false while no task is
// handed or the last load did not hold it.
func (view *View) shownTask() (shown *task.Task, ok bool) {
	shown, ok = view.tasks[view.taskID]
	return shown, ok
}

// layOut sizes the description below the fields and renders it again when the
// task or the width changed.
func (view *View) layOut() {
	shown, ok := view.shownTask()
	if !ok {
		return
	}
	view.description.SetWidth(view.width)
	view.description.SetHeight(max(view.height-len(view.renderFields(shown)), 0))
	isSameTask := shown.ID == view.renderedTaskID
	if isSameTask && shown.Description == view.renderedDescription && view.width == view.renderedWidth {
		return
	}
	view.renderedTaskID = shown.ID
	view.renderedDescription = shown.Description
	view.renderedWidth = view.width
	offset := view.description.YOffset()
	view.description.SetContent(view.renderDescription(shown.Description))
	if isSameTask {
		view.description.SetYOffset(offset)
	} else {
		view.description.GotoTop()
	}
}

func (view *View) Render() string {
	if view.height <= 0 {
		return ""
	}
	shown, ok := view.shownTask()
	switch {
	case view.taskID == "":
		return view.styles.Muted.Render("Nothing selected")
	case !ok && !view.hasTasks:
		return view.styles.Muted.Render("Loading tasks" + cells.Ellipsis)
	case !ok:
		return view.styles.Muted.Render("Task " + task.ShortID(view.taskID) + " is gone")
	}
	lines := view.renderFields(shown)
	if view.description.Height() > 0 {
		lines = append(lines, view.description.View())
	}
	return strings.Join(lines, "\n")
}

// renderFields returns a line per field of the task and the blank lines below
// them. A View too short to keep the description lines below them gets one
// line with every field and no blank lines.
func (view *View) renderFields(shown *task.Task) []string {
	fields := taskFields(shown)
	if len(fields)+descriptionGapLines+minDescriptionLines > view.height {
		return []string{view.renderFoldedFields(fields)}
	}
	labelWidth := labelColumnWidth()
	lines := make([]string, 0, len(fields)+descriptionGapLines)
	for _, entry := range fields {
		label := view.styles.Muted.Render(cells.Fit(entry.label, labelWidth))
		lines = append(lines, ansi.Truncate(label+labelGap+view.renderValue(entry), view.width, cells.Ellipsis))
	}
	for range descriptionGapLines {
		lines = append(lines, "")
	}
	return lines
}

func labelColumnWidth() int {
	width := 0
	for _, label := range fieldLabels {
		width = max(width, ansi.StringWidth(label))
	}
	return width
}

// renderFoldedFields renders the values of every field on one line, cut at
// the width of the View.
func (view *View) renderFoldedFields(fields []field) string {
	values := make([]string, 0, len(fields))
	for _, entry := range fields {
		values = append(values, view.renderValue(entry))
	}
	separator := view.styles.Muted.Render(foldedSeparator)
	return ansi.Truncate(strings.Join(values, separator), view.width, cells.Ellipsis)
}

// renderValue styles the value of the status and the ID fields the way the
// task list does.
func (view *View) renderValue(entry field) string {
	switch entry.label {
	case statusLabel:
		return view.styles.Status(task.TaskStatus(entry.value)).Render(entry.value)
	case idLabel:
		return view.styles.ShortID.Render(entry.value)
	}
	return entry.value
}

// renderDescription renders a markdown description wrapped at the width of
// the View. A description glamour fails to render shows as plain text below
// the error.
func (view *View) renderDescription(description string) string {
	if strings.TrimSpace(description) == "" {
		return view.styles.Muted.Render("No description")
	}
	rendered, err := renderMarkdown(description, view.width)
	if err != nil {
		return view.styles.Error.Render("Could not render the description as markdown: "+err.Error()) + "\n" + description
	}
	return rendered
}

func renderMarkdown(markdown string, width int) (string, error) {
	renderer, err := glamour.NewTermRenderer(glamour.WithStyles(markdownStyle()), glamour.WithWordWrap(width))
	if err != nil {
		return "", err
	}
	rendered, err := renderer.Render(markdown)
	if err != nil {
		return "", err
	}
	return strings.Trim(rendered, "\n"), nil
}

// markdownStyle returns the dark glamour style without the margin around the
// document, which the Tile border already gives.
func markdownStyle() glamouransi.StyleConfig {
	style := glamourstyles.DarkStyleConfig
	noMargin := uint(0)
	style.Document.Margin = &noMargin
	return style
}

// taskFields returns the fields of a task that are set, in the order of the
// task file.
func taskFields(shown *task.Task) []field {
	fields := []field{
		{label: titleLabel, value: shown.Title},
		{label: idLabel, value: task.ShortID(shown.ID)},
		{label: statusLabel, value: string(shown.Status)},
		{label: ticketLabel, value: shown.TicketID},
		{label: parentLabel, value: task.ShortID(shown.ParentTaskID)},
		{label: blockedByLabel, value: joinShortIDs(shown.BlockedBy)},
		{label: repositoriesLabel, value: strings.Join(shown.Repositories, listSeparator)},
		{label: pullRequestsLabel, value: strings.Join(shown.PullRequests, listSeparator)},
		{label: createdLabel, value: formatTime(shown.CreatedAt)},
		{label: updatedLabel, value: formatTime(shown.UpdatedAt)},
		{label: startedLabel, value: formatTime(shown.StartedAt)},
		{label: finishedLabel, value: formatTime(shown.FinishedAt)},
	}
	set := make([]field, 0, len(fields))
	for _, entry := range fields {
		if entry.value != "" {
			set = append(set, entry)
		}
	}
	return set
}

func joinShortIDs(ids []task.TaskID) string {
	shortIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		shortIDs = append(shortIDs, task.ShortID(id))
	}
	return strings.Join(shortIDs, listSeparator)
}

// formatTime returns an empty string for a time that is not set.
func formatTime(moment time.Time) string {
	if moment.IsZero() {
		return ""
	}
	return moment.Format(fieldTimeLayout)
}
