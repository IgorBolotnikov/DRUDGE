package cmd

import (
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const (
	doneGlyph   = "✓"
	skipGlyph   = "·"
	stepGlyph   = "›"
	warnGlyph   = "!"
	failedGlyph = "✗"
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
	// isBlankPending is set after a block. The blank line prints before the
	// next line of the group and is dropped when a header or a result comes
	// next.
	isBlankPending bool
	fields         []printerField
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
	p.isBlankPending = false
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

// failed prints a line about work that failed.
func (p *printer) failed(format string, args ...any) {
	p.glyphLine(theme.RoleError, failedGlyph, format, args...)
}

// detail prints a dimmed line one level deeper than the lines around it. The
// ANSI escape codes of the text are stripped first.
func (p *printer) detail(format string, args ...any) {
	text := common.StripANSI(fmt.Sprintf(format, args...))
	p.info("%s%s", detailIndent, p.theme.Paint(theme.Stdout, theme.RoleMuted, text))
}

// block prints text at the detail indent, one line per line of the text,
// with a blank line before and after it. The newlines that open and close the
// text add no line, and an empty text prints nothing.
func (p *printer) block(text string) {
	text = strings.Trim(text, "\n")
	if text == "" {
		return
	}
	p.flush()
	if p.hasPrinted {
		p.isBlankPending = true
	}
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			p.log.Info("")
			continue
		}
		p.info("%s%s", detailIndent, line)
	}
	p.isBlankPending = true
}

// field adds a labelled value at the detail indent. It prints with the next
// line that is not a field or on flush, so the values of consecutive fields
// line up. A field with no value prints its label alone.
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
		label := p.theme.Paint(theme.Stdout, theme.RoleMuted, pending.label)
		if pending.value == "" {
			p.info("%s%s", detailIndent, label)
			continue
		}
		padding := strings.Repeat(" ", labelWidth-len([]rune(pending.label)))
		p.info("%s%s%s%s%s", detailIndent, label, padding, fieldGap, pending.value)
	}
}

// task names a task in a line printed to stdout: the bold short id, two
// spaces and the title.
func (p *printer) task(named *task.Task) string {
	return p.taskID(named.ID) + taskTitleGap + named.Title
}

// taskID names a task by its bold short id in a line printed to stdout, where
// the title is not known.
func (p *printer) taskID(id task.TaskID) string {
	return p.theme.Bold(theme.Stdout, task.ShortID(id))
}

// warn prints a warning to stderr.
func (p *printer) warn(format string, args ...any) {
	p.flush()
	p.printPendingBlank()
	p.indentedLog().Warn(format, args...)
	p.hasPrinted = true
}

// ask prints a question at column 0 to stdout and leaves the cursor at the
// end of it for the answer.
func (p *printer) ask(question string) {
	p.closeGroup()
	fmt.Print(question)
	p.hasPrinted = true
}

// result closes the group and prints the outcome at column 0.
func (p *printer) result(format string, args ...any) {
	p.closeGroup()
	p.done(format, args...)
}

// resultWarn closes the group and prints at column 0 an outcome that went
// wrong while the command goes on.
func (p *printer) resultWarn(format string, args ...any) {
	p.closeGroup()
	p.glyphLine(theme.RoleWarning, warnGlyph, format, args...)
}

// resultFailed closes the group and prints at column 0 an outcome of work
// that failed.
func (p *printer) resultFailed(format string, args ...any) {
	p.closeGroup()
	p.failed(format, args...)
}

// skipResult closes the group and prints at column 0 that there was nothing
// to do.
func (p *printer) skipResult(format string, args ...any) {
	p.closeGroup()
	p.skip(format, args...)
}

// view closes the group and prints lines at column 0 as they are. A blank
// line goes before them when anything was printed before.
func (p *printer) view(lines []string) {
	p.closeGroup()
	if p.hasPrinted {
		p.log.Info("")
	}
	for _, line := range lines {
		// The line is already formatted and may hold a percent sign.
		p.info("%s", line)
	}
}

func (p *printer) closeGroup() {
	p.flush()
	p.isBlankPending = false
	p.isGroupOpen = false
}

func (p *printer) glyphLine(role string, glyph string, format string, args ...any) {
	p.info("%s %s", p.theme.Paint(theme.Stdout, role, glyph), fmt.Sprintf(format, args...))
}

func (p *printer) info(format string, args ...any) {
	p.flush()
	p.printPendingBlank()
	p.indentedLog().Info(format, args...)
	p.hasPrinted = true
}

func (p *printer) printPendingBlank() {
	if p.isBlankPending {
		p.log.Info("")
		p.isBlankPending = false
	}
}

func (p *printer) indentedLog() *common.Logger {
	if !p.isGroupOpen {
		return p.log
	}
	return p.log.Indented(groupIndent)
}
