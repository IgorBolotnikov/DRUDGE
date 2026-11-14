package tui

import (
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/report"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// eventLogCap is how many entries the event log keeps. An entry past it drops
// the oldest one.
const eventLogCap = 1000

const (
	eventLogTitle      = "Event log"
	eventLogTimeLayout = "15:04"
	eventLogSeparator  = "  "
	emptyEventLogText  = "No command reported a warning or an error yet."
)

// eventLogScrollKey stands for the scroll keys of the event log in the help.
var eventLogScrollKey = key.NewBinding(key.WithKeys("k", "j", "up", "down"), key.WithHelp("j/k", "scroll"))

// eventLogEntry is one warning or error a command reported.
type eventLogEntry struct {
	at      time.Time
	command actions.Command
	level   report.Level
	text    string
}

// eventLog holds the warnings and the errors of every command for the life of
// the TUI, oldest first.
type eventLog struct {
	entries []eventLogEntry
}

func (events *eventLog) add(entry eventLogEntry) {
	if len(events.entries) == eventLogCap {
		events.entries = slices.Delete(events.entries, 0, 1)
	}
	events.entries = append(events.entries, entry)
}

// eventLogOverlay is the box that lists the entries of the event log over the
// space of a Tab. It follows new entries while it is scrolled to the bottom.
type eventLogOverlay struct {
	events       *eventLog
	styles       *styles.Styles
	content      viewport.Model
	screenHeight int
}

func newEventLogOverlay(events *eventLog, overlayStyles *styles.Styles) *eventLogOverlay {
	content := viewport.New()
	content.SoftWrap = true
	return &eventLogOverlay{events: events, styles: overlayStyles, content: content}
}

// SetSize fits the box in the space of a Tab within the margins of a View
// overlay. The title takes the first line inside the box.
func (overlay *eventLogOverlay) SetSize(screenWidth, screenHeight int) {
	overlay.screenHeight = screenHeight
	box := overlay.styles.HelpBox
	tabHeight := max(screenHeight-barHeight, 0)
	overlay.content.SetWidth(max(screenWidth-2*viewOverlayMarginX-box.GetHorizontalFrameSize(), 0))
	overlay.content.SetHeight(max(tabHeight-2*viewOverlayMarginY-box.GetVerticalFrameSize()-1, 0))
}

func (overlay *eventLogOverlay) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	overlay.content, cmd = overlay.content.Update(msg)
	return cmd
}

// Render returns an empty box while the box has no room for an entry.
func (overlay *eventLogOverlay) Render() (box string, x, y int) {
	if overlay.content.Width() == 0 || overlay.content.Height() == 0 {
		return "", 0, 0
	}
	isAtBottom := overlay.content.AtBottom()
	overlay.content.SetContent(overlay.renderEntries())
	if isAtBottom {
		overlay.content.GotoBottom()
	}
	title := cells.Fit(overlay.styles.HelpTitle.Render(eventLogTitle), overlay.content.Width())
	box = overlay.styles.HelpBox.Render(lipgloss.JoinVertical(lipgloss.Left, title, overlay.content.View()))
	return box, viewOverlayMarginX, tabTop + viewOverlayMarginY
}

// renderEntries renders one entry per line, like 14:05  run 3f9a1c2e  text.
func (overlay *eventLogOverlay) renderEntries() string {
	if len(overlay.events.entries) == 0 {
		return overlay.styles.Muted.Render(emptyEventLogText)
	}
	lines := make([]string, 0, len(overlay.events.entries))
	for _, entry := range overlay.events.entries {
		textStyle := overlay.styles.Warning
		if entry.level == report.LevelError {
			textStyle = overlay.styles.Error
		}
		lines = append(lines, strings.Join([]string{
			overlay.styles.Muted.Render(entry.at.Format(eventLogTimeLayout)),
			entry.command.String(),
			textStyle.Render(entry.text),
		}, eventLogSeparator))
	}
	return strings.Join(lines, "\n")
}

// Mouse scrolls the entries on the wheel.
func (overlay *eventLogOverlay) Mouse(msg tea.MouseMsg) tea.Cmd {
	return overlay.Update(msg)
}

func (overlay *eventLogOverlay) Title() string {
	return eventLogTitle
}

func (overlay *eventLogOverlay) Keys() []key.Binding {
	return []key.Binding{eventLogScrollKey}
}

func (overlay *eventLogOverlay) ShortHelp() []key.Binding {
	return []key.Binding{closeOverlayKey, helpKey}
}

func (overlay *eventLogOverlay) TakesAllKeys() bool {
	return false
}

func (overlay *eventLogOverlay) Feeds() []views.Feed {
	return nil
}
