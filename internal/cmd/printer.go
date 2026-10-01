package cmd

import (
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const (
	doneGlyph = "✓"
	skipGlyph = "·"
)

// groupIndent is the indent of a line inside an open group.
const groupIndent = "  "

// printer prints the lines of a command as groups. A header opens a group
// and a result closes it.
type printer struct {
	log         *common.Logger
	theme       *theme.Theme
	hasPrinted  bool
	isGroupOpen bool
}

func newPrinter(log *common.Logger, palette *theme.Theme) *printer {
	return &printer{log: log, theme: palette}
}

// header prints a bold line at column 0 and opens a group. A blank line goes
// before it when anything was printed before.
func (p *printer) header(format string, args ...any) {
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

// warn prints a warning to stderr.
func (p *printer) warn(format string, args ...any) {
	p.indentedLog().Warn(format, args...)
	p.hasPrinted = true
}

// result closes the group and prints the outcome at column 0.
func (p *printer) result(format string, args ...any) {
	p.isGroupOpen = false
	p.done(format, args...)
}

func (p *printer) glyphLine(role string, glyph string, format string, args ...any) {
	p.info("%s %s", p.theme.Paint(theme.Stdout, role, glyph), fmt.Sprintf(format, args...))
}

func (p *printer) info(format string, args ...any) {
	p.indentedLog().Info(format, args...)
	p.hasPrinted = true
}

func (p *printer) indentedLog() *common.Logger {
	if !p.isGroupOpen {
		return p.log
	}
	return p.log.Indented(groupIndent)
}
