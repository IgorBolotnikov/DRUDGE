// Package taskrunlog is the View that shows the latest run of one task the
// way drg task runlog does and follows it while it runs.
package taskrunlog

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/style"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Name is the kind of the View that shows the run of a task.
const Name = "task-runlog"

// Description says in one line what the View shows.
const Description = "Shows the compact runlog of a task opened with L or selected in a task list or a task board of the same tab, and follows the run while it writes."

// Title of the Tile of the View: the name of the View, the short ID of the
// task and the title of the task once the View read it.
const (
	titleFormat         = "%s [%s]"
	titleWithTaskFormat = "%s [%s] %s"
)

// Status segments of the View.
const (
	followingStatus = "following"
	pausedStatus    = "paused"
)

// Keys of the runlog. The scroll up key and the top key carry the help of
// their pair.
var (
	scrollUpKey     = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "scroll"))
	scrollDownKey   = key.NewBinding(key.WithKeys("j", "down"))
	halfPageUpKey   = key.NewBinding(key.WithKeys("u", "pgup"), key.WithHelp("u/d", "half page up/down"))
	halfPageDownKey = key.NewBinding(key.WithKeys("d", "pgdown"))
	scrollTopKey    = key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g/G", "top/bottom"))
	scrollBottomKey = key.NewBinding(key.WithKeys("G", "end"))
)

// scrollLineCount is how many lines the scroll keys and one turn of the wheel
// move the runlog.
const scrollLineCount = 1

// Keys are the keys of the runlog the help lists.
var Keys = slices.Concat([]key.Binding{scrollUpKey, halfPageUpKey, scrollTopKey}, views.TaskCommandKeys)

// View shows the steps and the stderr lines of the latest run of a task. It
// appends what each read of the runlog feed found. It follows the run at the
// bottom until it is scrolled up, and follows again once it is scrolled back
// to the bottom.
type View struct {
	styles  *styles.Styles
	taskID  task.TaskID
	content viewport.Model
	// hasRead is whether a read of the run worked. The fields below hold
	// what the reads found.
	hasRead bool
	readErr error
	shown   *task.Task
	runDir  string
	hasRun  bool
	// harness and isDecoded tell whether DRUDGE decodes the event stream of
	// the run.
	harness         drudger.Harness
	isDecoded       bool
	isStreamMissing bool
	steps           []drudger.Step
	stderr          []string
	isFollowing     bool
	// blockStarts holds the index of the first wrapped line of every block
	// of the content.
	blockStarts []int
}

// New returns a runlog that shows nothing until it is opened on a task.
func New(viewStyles *styles.Styles) *View {
	return &View{styles: viewStyles, content: viewport.New(), isFollowing: true}
}

func (view *View) Name() string {
	return Name
}

// Title returns the name of the View, the short ID of the task and its title,
// like task-runlog [3f9a1c2e] Add retry to uploader. The title of the task
// shows once a read of the run worked.
func (view *View) Title() string {
	if view.taskID == "" {
		return Name
	}
	shortID := task.ShortID(view.taskID)
	if view.shown == nil {
		return fmt.Sprintf(titleFormat, Name, shortID)
	}
	return fmt.Sprintf(titleWithTaskFormat, Name, shortID, view.shown.Title)
}

// TakesAllKeys reports false. The View has no input that takes keys.
func (view *View) TakesAllKeys() bool {
	return false
}

// Status returns whether the View follows the run or is paused. It is empty
// while no task is shown.
func (view *View) Status() string {
	switch {
	case view.taskID == "":
		return ""
	case view.isFollowing:
		return followingStatus
	}
	return pausedStatus
}

// ShowTask shows the run of the task with id, following it at the bottom. It
// drops the steps of the task it showed before. Showing the task it shows
// keeps what it shows.
func (view *View) ShowTask(id task.TaskID) {
	if id == view.taskID {
		return
	}
	*view = View{styles: view.styles, taskID: id, content: view.content, isFollowing: true}
	view.renderContent()
}

// Open shows the run of the task of an open runlog message the way showing
// the task does. It refuses every other open message.
func (view *View) Open(msg views.OpenMsg) (isShown bool) {
	openRunLog, ok := msg.(views.OpenRunLogMsg)
	if !ok {
		return false
	}
	view.ShowTask(openRunLog.TaskID)
	return true
}

// Feeds returns the runlog feed of the shown task, or no feed while no task
// is shown. Showing another task swaps the feed.
func (view *View) Feeds() []views.Feed {
	if view.taskID == "" {
		return nil
	}
	return []views.Feed{{Name: views.RunLogFeed, TaskID: view.taskID}}
}

// SetSize wraps the run again on a new width. A paused View keeps the wrapped
// line at its top in place.
func (view *View) SetSize(width, height int) {
	isNewWidth := width != view.content.Width()
	blockIndex, lineIndex := view.topLine()
	view.content.SetWidth(width)
	view.content.SetHeight(height)
	if isNewWidth {
		view.renderContent()
	}
	if view.isFollowing {
		view.content.GotoBottom()
		return
	}
	if isNewWidth {
		view.scrollToLine(blockIndex, lineIndex)
	}
}

// Update takes the reads of the run and scrolls on keys and on the wheel. The
// task command keys ask for their command on the shown task.
func (view *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case views.RunStepsReadMsg:
		view.handleRead(msg)
	case tea.KeyPressMsg:
		if cmd := views.TaskCommand(msg, view.taskID); cmd != nil {
			return cmd
		}
		view.handleKey(msg)
	case tea.MouseWheelMsg:
		view.handleWheel(msg)
	}
	return nil
}

// handleRead appends what a read of the run found. The first read of the
// feed and a read of a rerun replace what the View shows. A rerun scrolls to
// the top while the View is paused. A failed read keeps what the View shows.
func (view *View) handleRead(msg views.RunStepsReadMsg) {
	if msg.TaskID != view.taskID {
		return
	}
	view.readErr = msg.Err
	if msg.Err != nil {
		view.renderContent()
		return
	}
	read := msg.Read
	if msg.IsFirstRead || read.IsReset {
		view.steps = nil
		view.stderr = nil
	}
	view.hasRead = true
	view.shown = read.Task
	view.runDir = read.RunDir
	view.hasRun = read.HasRun
	view.harness = read.Harness
	view.isDecoded = read.IsDecoded
	view.isStreamMissing = read.IsStreamMissing
	view.steps = slices.Concat(view.steps, read.Steps)
	view.stderr = slices.Concat(view.stderr, read.Stderr)
	view.renderContent()
	if read.IsReset && !view.isFollowing {
		view.content.GotoTop()
	}
}

func (view *View) handleKey(msg tea.KeyPressMsg) {
	switch {
	case key.Matches(msg, scrollUpKey):
		view.content.ScrollUp(scrollLineCount)
	case key.Matches(msg, scrollDownKey):
		view.content.ScrollDown(scrollLineCount)
	case key.Matches(msg, halfPageUpKey):
		view.content.HalfPageUp()
	case key.Matches(msg, halfPageDownKey):
		view.content.HalfPageDown()
	case key.Matches(msg, scrollTopKey):
		view.content.GotoTop()
	case key.Matches(msg, scrollBottomKey):
		view.content.GotoBottom()
	default:
		return
	}
	view.isFollowing = view.content.AtBottom()
}

func (view *View) handleWheel(msg tea.MouseWheelMsg) {
	switch msg.Button {
	case tea.MouseWheelUp:
		view.content.ScrollUp(scrollLineCount)
	case tea.MouseWheelDown:
		view.content.ScrollDown(scrollLineCount)
	default:
		return
	}
	view.isFollowing = view.content.AtBottom()
}

func (view *View) Render() string {
	return view.content.View()
}

// minWrapWidth is the fewest cells a wrapped line keeps for text. A line
// whose hanging indent leaves less wraps with no indent.
const minWrapWidth = 10

// The wrap helpers always break after a hyphen, which splits a flag like -run
// from its dash. A line wraps with every hyphen swapped for a non-breaking
// hyphen of the same width and gets its hyphens back after.
const (
	hyphen            = "-"
	nonBreakingHyphen = "\u2011"
)

// block is a piece of the runlog that wraps on its own: a line about the run,
// a step or a stderr line. Every line of a block after the first and every
// continuation of a wrapped line starts with its hanging indent.
type block struct {
	lines         []string
	hangingIndent string
}

// renderContent wraps the lines of the run to the width of the View into the
// scrolled content and keeps the bottom in sight while the View follows the
// run.
func (view *View) renderContent() {
	var lines []string
	view.blockStarts = nil
	for _, runBlock := range view.renderBlocks() {
		view.blockStarts = append(view.blockStarts, len(lines))
		for _, line := range runBlock.lines {
			lines = append(lines, wrapLine(line, runBlock.hangingIndent, view.content.Width())...)
		}
	}
	view.content.SetContentLines(lines)
	if view.isFollowing {
		view.content.GotoBottom()
	}
}

// topLine returns the block at the top of the View and the wrapped line of it
// at the top.
func (view *View) topLine() (blockIndex, lineIndex int) {
	if len(view.blockStarts) == 0 {
		return 0, 0
	}
	offset := view.content.YOffset()
	for index, start := range view.blockStarts {
		if start > offset {
			break
		}
		blockIndex = index
	}
	return blockIndex, offset - view.blockStarts[blockIndex]
}

// scrollToLine scrolls the wrapped line of a block to the top of the View. A
// line past the end of the block, gone after a widening, scrolls its last
// line to the top.
func (view *View) scrollToLine(blockIndex, lineIndex int) {
	if blockIndex >= len(view.blockStarts) {
		return
	}
	nextStart := view.content.TotalLineCount()
	if blockIndex+1 < len(view.blockStarts) {
		nextStart = view.blockStarts[blockIndex+1]
	}
	view.content.SetYOffset(min(view.blockStarts[blockIndex]+lineIndex, nextStart-1))
}

// wrapLine wraps a line at word boundaries to width and breaks a word wider
// than width. A line breaks only at spaces. The continuation lines start with the hanging indent. A width
// that leaves the text less than the minimum wrap width drops the indent.
func wrapLine(line, hangingIndent string, width int) []string {
	indentWidth := ansi.StringWidth(hangingIndent)
	if width-indentWidth < minWrapWidth {
		hangingIndent = ""
		indentWidth = 0
	}
	head := ansi.Cut(line, 0, indentWidth)
	text := ansi.Cut(line, indentWidth, ansi.StringWidth(line))
	text = strings.ReplaceAll(text, hyphen, nonBreakingHyphen)
	text = lipgloss.Wrap(text, width-indentWidth, "")
	wrapped := strings.Split(strings.ReplaceAll(text, nonBreakingHyphen, hyphen), "\n")
	wrapped[0] = head + wrapped[0]
	for index := 1; index < len(wrapped); index++ {
		wrapped[index] = hangingIndent + wrapped[index]
	}
	return wrapped
}

// renderBlocks returns the run directory, the notes about the event stream,
// every step and the stderr lines below them. A View with no read of the run
// returns the line that says why.
func (view *View) renderBlocks() []block {
	switch {
	case view.taskID == "":
		return []block{lineBlock(view.styles.Muted.Render("No task selected"))}
	case !view.hasRead && view.readErr != nil:
		return []block{lineBlock(view.styles.Error.Render("Could not read the run: " + view.readErr.Error()))}
	case !view.hasRead:
		return []block{lineBlock(view.styles.Muted.Render("Reading the run" + cells.Ellipsis))}
	case !view.hasRun:
		return []block{lineBlock(view.styles.Muted.Render("No run yet, its steps show here once the task runs"))}
	}

	blocks := []block{lineBlock(view.styles.Muted.Render(view.runDir))}
	if !view.isDecoded {
		blocks = append(blocks, lineBlock(view.styles.Muted.Render(fmt.Sprintf(style.NoDecoderNote, view.harness))))
	}
	if view.isStreamMissing {
		blocks = append(blocks, lineBlock(style.NotWrittenYetLabel))
	}
	for _, step := range view.steps {
		blocks = append(blocks, view.renderStep(step))
	}
	if len(view.stderr) > 0 {
		blocks = append(blocks, lineBlock(""), lineBlock(style.StderrHeading))
		for _, line := range view.stderr {
			blocks = append(blocks, lineBlock(view.styles.Error.Render(line)))
		}
	}
	return blocks
}

func lineBlock(line string) block {
	return block{lines: []string{line}}
}

// renderStep paints the segments of a step into its lines. The first line is
// indented by the depth of the step and every other line of agent text by its
// hanging indent.
func (view *View) renderStep(step drudger.Step) block {
	hangingIndent := style.StepHangingIndent(step)
	lines := []string{style.StepIndent(step.Depth)}
	for _, segment := range style.StepSegments(step) {
		segmentStyle := view.styles.Role(segment.Role)
		for index, part := range strings.Split(segment.Text, "\n") {
			if index > 0 {
				lines = append(lines, hangingIndent)
			}
			lines[len(lines)-1] += segmentStyle.Render(part)
		}
	}
	return block{lines: lines, hangingIndent: hangingIndent}
}
