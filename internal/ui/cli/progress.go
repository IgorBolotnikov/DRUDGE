package cli

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/cli/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/report"
)

// cliProgress renders the events a domain service reports as the lines drg
// prints.
type cliProgress struct {
	out *printer.Printer
}

// newCLIProgress builds a progress port that prints through out.
func newCLIProgress(out *printer.Printer) *cliProgress {
	return &cliProgress{out: out}
}

// Report prints the lines of an event. The fields an event ends with print
// right away.
func (p *cliProgress) Report(event any) {
	lines := report.Lines(event, p.out.Task)
	for _, line := range lines {
		p.printLine(line)
	}
	if len(lines) > 0 {
		p.out.Flush()
	}
}

func (p *cliProgress) printLine(line report.Line) {
	switch line.Kind {
	case report.KindHeader:
		p.out.Header("%s", line.Text)
	case report.KindLine:
		p.groupLine(line.Level)("%s", line.Text)
	case report.KindResult:
		p.resultLine(line.Level)("%s", line.Text)
	case report.KindDetail:
		p.out.Detail("%s", line.Text)
	case report.KindField:
		p.out.Field(line.Label, line.Text)
	case report.KindBlock:
		p.out.Block(line.Text)
	}
}

// groupLine picks the printer method of a line inside a group by its level.
func (p *cliProgress) groupLine(level report.Level) func(format string, args ...any) {
	switch level {
	case report.LevelStep:
		return p.out.Step
	case report.LevelSkipped:
		return p.out.Skip
	case report.LevelWarning:
		return p.out.Warn
	case report.LevelError:
		return p.out.Failed
	default:
		return p.out.Done
	}
}

// resultLine picks the printer method of a result line by its level.
func (p *cliProgress) resultLine(level report.Level) func(format string, args ...any) {
	switch level {
	case report.LevelSkipped:
		return p.out.SkipResult
	case report.LevelWarning:
		return p.out.ResultWarn
	case report.LevelError:
		return p.out.ResultFailed
	default:
		return p.out.Result
	}
}
