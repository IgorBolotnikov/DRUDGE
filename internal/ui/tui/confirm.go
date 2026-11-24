package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const confirmTitle = "Confirm"

// Keys of the confirm overlay.
var (
	confirmYesKey = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes"))
	confirmNoKey  = key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "no"))
)

var confirmBindings = []key.Binding{confirmYesKey, confirmNoKey}

// confirmOverlay is the box that shows a question centered over the screen
// and sends the answer on its answer channel. It sends one answer at most, so
// the channel needs room for one answer. The root passes it keys and closes
// it once it has the answer.
type confirmOverlay struct {
	question     string
	answer       chan<- bool
	isAnswered   bool
	help         help.Model
	boxStyle     lipgloss.Style
	screenWidth  int
	screenHeight int
}

func newConfirmOverlay(question string, answer chan<- bool, keyHelp help.Model, overlayStyles *styles.Styles) *confirmOverlay {
	return &confirmOverlay{question: question, answer: answer, help: keyHelp, boxStyle: overlayStyles.HelpBox}
}

// answerKey answers yes on y and no on n or esc, and reports whether the key
// answered. Every other key does nothing.
func (overlay *confirmOverlay) answerKey(msg tea.KeyPressMsg) bool {
	switch {
	case key.Matches(msg, confirmYesKey):
		overlay.send(true)
	case key.Matches(msg, confirmNoKey):
		overlay.send(false)
	default:
		return false
	}
	return true
}

// cancel answers no unless the overlay already answered.
func (overlay *confirmOverlay) cancel() {
	overlay.send(false)
}

func (overlay *confirmOverlay) send(isYes bool) {
	if overlay.isAnswered {
		return
	}
	overlay.isAnswered = true
	overlay.answer <- isYes
}

func (overlay *confirmOverlay) SetSize(screenWidth, screenHeight int) {
	overlay.screenWidth = screenWidth
	overlay.screenHeight = screenHeight
}

func (overlay *confirmOverlay) Update(msg tea.Msg) tea.Cmd {
	return nil
}

// Render centers the box with the question and its keys on the screen.
func (overlay *confirmOverlay) Render() (box string, x, y int) {
	content := lipgloss.JoinVertical(lipgloss.Left, overlay.question, "", overlay.help.ShortHelpView(confirmBindings))
	box = overlay.boxStyle.Render(content)
	x = max((overlay.screenWidth-lipgloss.Width(box))/2, 0)
	y = max((overlay.screenHeight-lipgloss.Height(box))/2, 0)
	return box, x, y
}

func (overlay *confirmOverlay) Mouse(msg tea.MouseMsg) tea.Cmd {
	return nil
}

func (overlay *confirmOverlay) Title() string {
	return confirmTitle
}

func (overlay *confirmOverlay) Keys() []key.Binding {
	return confirmBindings
}

func (overlay *confirmOverlay) ShortHelp() []key.Binding {
	return []key.Binding{confirmYesKey, confirmNoKey, helpKey}
}

func (overlay *confirmOverlay) TakesAllKeys() bool {
	return false
}

func (overlay *confirmOverlay) Feeds() []views.Feed {
	return nil
}
