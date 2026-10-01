package cmd

import (
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const (
	doneGlyph = "✓"
	skipGlyph = "·"
	stepGlyph = "›"
)

// stepEnding ends the line of a step that started.
const stepEnding = "…"

// groupIndent is the indent of a line inside an open group.
const groupIndent = "  "

// detailIndent is how much deeper than the lines around it a detail line or
// a field sits.
const detailIndent = groupIndent + groupIndent

// fieldGap is the space between the longest label of a run of fields and its
// value.
const fieldGap = "  "

// taskTitleGap separates the short id of a task from its title.
const taskTitleGap = "  "

// printer prints the lines of a command as groups. A header opens a group
// and a result closes it.
type printer struct {
	log         *common.Logger
	theme       *theme.Theme
	hasPrinted  bool
	isGroupOpen bool
	fields      []printerField
}

type printerField struct {
	label string
	value string
}

func newPrinter(log *common.Logger, palette *theme.Theme) *printer {
	return &printer{log: log, theme: palette}
}

// header prints a bold line at column 0 and opens a group. A blank line goes
// before it when anything was printed before.
func (p *printer) header(format string, args ...any) {
	p.flush()
	if p.hasPrinted {
		p.log.Info("")
	}
	p.isGroupOpen = false
	p.info("%s", p.theme.Bold(theme.Stdout, fmt.Sprintf(format, args...)))
	p.isGroupOpen = true
}

// done prints a line about a thing that got done.
func (p *printer) done(format string, args ...any) {
	p.glyphLine(theme.RoleSuccess, doneGlyph, format, args...)
}

// skip prints a line about a thing there was nothing to do for.
func (p *printer) skip(format string, args ...any) {
	p.glyphLine(theme.RoleMuted, skipGlyph, format, args...)
}

// step prints a line about a slow step that started.
func (p *printer) step(format string, args ...any) {
	p.glyphLine(theme.RoleMuted, stepGlyph, "%s%s", fmt.Sprintf(format, args...), stepEnding)
}

// detail prints a dimmed line one level deeper than the lines around it. The
// ANSI escape codes of the text are stripped first.
func (p *printer) detail(format string, args ...any) {
	text := common.StripANSI(fmt.Sprintf(format, args...))
	p.info("%s%s", detailIndent, p.theme.Paint(theme.Stdout, theme.RoleMuted, text))
}

// field adds a labelled value at the detail indent. It prints with the next
// line that is not a field or on flush, so the values of consecutive fields
// line up.
func (p *printer) field(label string, value string) {
	p.fields = append(p.fields, printerField{label: label, value: value})
}

// flush prints the fields that wait for the end of their run.
func (p *printer) flush() {
	fields := p.fields
	p.fields = nil
	labelWidth := 0
	for _, pending := range fields {
		labelWidth = max(labelWidth, len([]rune(pending.label)))
	}
	for _, pending := range fields {
		padding := strings.Repeat(" ", labelWidth-len([]rune(pending.label)))
		label := p.theme.Paint(theme.Stdout, theme.RoleMuted, pending.label)
		p.info("%s%s%s%s%s", detailIndent, label, padding, fieldGap, pending.value)
	}
}

// task names a task in a line printed to stdout: the bold short id, two
// spaces and the title.
func (p *printer) task(named *task.Task) string {
	return p.theme.Bold(theme.Stdout, task.ShortID(named.ID)) + taskTitleGap + named.Title
}

// warn prints a warning to stderr.
func (p *printer) warn(format string, args ...any) {
	p.flush()
	p.indentedLog().Warn(format, args...)
	p.hasPrinted = true
}

// result closes the group and prints the outcome at column 0.
func (p *printer) result(format string, args ...any) {
	p.flush()
	p.isGroupOpen = false
	p.done(format, args...)
}

func (p *printer) glyphLine(role string, glyph string, format string, args ...any) {
	p.info("%s %s", p.theme.Paint(theme.Stdout, role, glyph), fmt.Sprintf(format, args...))
}

func (p *printer) info(format string, args ...any) {
	p.flush()
	p.indentedLog().Info(format, args...)
	p.hasPrinted = true
}

func (p *printer) indentedLog() *common.Logger {
	if !p.isGroupOpen {
		return p.log
	}
	return p.log.Indented(groupIndent)
}
