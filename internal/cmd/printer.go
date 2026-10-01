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

// printer prints every line a command shows about its work. It owns the
// glyphs, the indents, the blank lines and the colors of that output. A
// command handler and the progress renderer print through it and never write
// to stdout or stderr themselves.
//
// # Lines
//
// Each line about the work starts with one glyph:
//
//	✓  printer.done    a thing got done
//	›  printer.step    a slow step started, the line ends with …
//	·  printer.skip    nothing to do, skipped or already there
//	!  printer.warn    something went wrong and the command goes on
//	✗  printer.failed  the work failed
//
// The format and the arguments work like in [fmt.Printf]. Only the glyph gets
// a color. A step that finished prints nothing: the next line says it worked.
//
// # Groups
//
// A command that does several things for one subject prints them as a group.
// [printer.header] prints a bold line at column 0 and opens the group. The
// lines that follow are indented two spaces. One of the result methods closes
// the group and prints the outcome at column 0:
//
//	printer.result        ✓ the work got done
//	printer.resultWarn    ! the work went wrong and the command goes on
//	printer.resultFailed  ✗ the work failed
//	printer.skipResult    · there was nothing to do
//
// Lines printed with no open group sit at column 0. A command that does one
// thing prints a single line and opens no group.
//
// # Details, fields and blocks
//
// [printer.detail] prints a dimmed line one level deeper than the lines around
// it. It is for subprocess output and strips its ANSI codes.
//
// [printer.field] adds a labelled value. Fields wait until the next line that
// is not a field, so the values of a run of fields line up. Fields at the end
// of a command print only on [printer.flush].
//
// [printer.block] prints text as it is at the detail indent, with a blank line
// before and after it. A prompt or a list of commands goes in a block.
//
// # Views and questions
//
// [printer.view] closes the group and prints lines that are already formatted,
// like a table. [printer.ask] closes the group and prints a question with no
// newline, for the answer to follow on the same line.
//
// # Blank lines
//
// The printer decides every blank line. One goes before a header, a block or a
// view when anything was printed before it. It never prints two blank lines in
// a row, a blank line first or a blank line last. A command never prints a
// blank line itself.
//
// # Streams
//
// Info lines go to stdout and warnings go to stderr. The theme decides color
// for each stream, so piped output keeps the glyphs and loses the color.
// [printer.task] and [printer.taskID] name a task with its bold short id, for
// a line that goes to stdout.
//
// # Example
//
// A task run prints through the printer like this:
//
//	out.header("Task %s → Drudger %d (%s)", out.task(picked), slot, sandbox)
//	out.step("Fetching main of api from origin")
//	out.warn("Could not fetch main of api: %v", err)
//	out.step("Starting the agent, waiting up to %s for its first output", grace)
//	out.result("Drudger %s is working on task %s", sandbox, out.task(picked))
//	out.field("Branch", branch)
//	out.field("Run dir", runDir)
//	out.flush()
//
// and the user sees:
//
//	Task 3f9a1c2e  Add retry to uploader → Drudger 2 (drudge-demo-2)
//	  › Fetching main of api from origin…
//	  ! Could not fetch main of api: exit status 128
//	  › Starting the agent, waiting up to 30s for its first output…
//	✓ Drudger drudge-demo-2 is working on task 3f9a1c2e  Add retry to uploader
//	    Branch   drudge/3f9a-add-retry
//	    Run dir  .drudge/runs/3f9a1c2e-…
//
// A domain service never gets a printer. It reports typed events through the
// progress port, and the progress renderer turns each event into one of the
// calls above.
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
// line goes before them when anything was printed before. Blank lines at the
// start and the end of the lines are dropped, and a run of blank lines prints
// as one.
func (p *printer) view(lines []string) {
	p.closeGroup()
	if p.hasPrinted {
		p.isBlankPending = true
	}
	for _, line := range lines {
		if line == "" {
			p.isBlankPending = p.hasPrinted
			continue
		}
		// The line is already formatted and may hold a percent sign.
		p.info("%s", line)
	}
	p.isBlankPending = false
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
