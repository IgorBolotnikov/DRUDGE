package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/term"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

// errCancelled is returned when the user cancels a question with Ctrl-C or Esc.
var errCancelled = errors.New("cancelled")

const (
	cancelledExitCode = 130
	failedExitCode    = 1
	// signalExitBase plus the number of a signal is the exit code of a
	// process the signal stopped.
	signalExitBase = 128
)

// ExitCode returns the exit code of drg for the error a command returned.
func ExitCode(err error) int {
	if errors.Is(err, errCancelled) {
		return cancelledExitCode
	}
	return failedExitCode
}

// Bytes the selector reads from the terminal.
const (
	ctrlCByte        = 0x03
	carriageReturn   = '\r'
	lineFeed         = '\n'
	escapeByte       = 0x1b
	upLetter         = 'k'
	downLetter       = 'j'
	escapeCSI        = '['
	escapeSS3        = 'O'
	arrowUpFinal     = 'A'
	arrowDownFinal   = 'B'
	escapeFinalFirst = 0x40
	escapeFinalLast  = 0x7e
)

// ANSI sequences the selector redraws its list with.
const (
	cursorUpFormat    = "\x1b[%dA"
	clearToScreenEnd  = "\x1b[J"
	selectorLineBreak = "\r\n"
	// lineBreak ends a line in cooked mode, where the terminal adds the
	// carriage return.
	lineBreak = "\n"
)

const (
	warningGlyph        = "!"
	currentOptionMarker = "› "
	otherOptionMarker   = "  "
	defaultOptionTag    = "(default)"
)

// keyReadSize is the most bytes one read from the terminal takes.
const keyReadSize = 64

type selectorKey int

const (
	keyUp selectorKey = iota
	keyDown
	keyPick
	keyCancel
)

// arrowKeys maps the final byte of an arrow key escape sequence to its key.
var arrowKeys = map[byte]selectorKey{arrowUpFinal: keyUp, arrowDownFinal: keyDown}

// isInteractive reports whether drg can ask questions, which takes a terminal
// on both stdin and stdout.
func isInteractive() bool {
	return common.IsTerminal(os.Stdin) && common.IsTerminal(os.Stdout)
}

// askQuestions asks every question whose condition holds on output, reads
// the answers from input and returns them. The terminal of input is in raw
// mode while a select waits for its pick, and in cooked mode while a typed
// question waits for its line.
func askQuestions(input io.Reader, output io.Writer, palette *theme.Theme, questions []common.Question) (common.Answers, error) {
	return newPrompter(input, output, palette).ask(questions)
}

// prompter asks questions on a terminal.
type prompter struct {
	input   io.Reader
	reader  *bufio.Reader
	keys    *keyReader // Reads from reader
	output  io.Writer
	palette *theme.Theme
	// interrupts returns the channel Ctrl-C arrives on while a typed question
	// waits for its line, and the function that stops it.
	interrupts func() (<-chan os.Signal, func())
}

func newPrompter(input io.Reader, output io.Writer, palette *theme.Theme) *prompter {
	reader := bufio.NewReader(input)
	return &prompter{input: input, reader: reader, keys: &keyReader{input: reader}, output: output, palette: palette, interrupts: notifyInterrupts}
}

func notifyInterrupts() (<-chan os.Signal, func()) {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	return interrupts, func() { signal.Stop(interrupts) }
}

func (p *prompter) ask(questions []common.Question) (common.Answers, error) {
	answers := make(common.Answers, len(questions))
	for _, question := range questions {
		if !question.IsAskedAfter(answers) {
			continue
		}
		var answer string
		var err error
		switch question.Kind {
		case common.QuestionSelect:
			answer, err = p.askSelect(question)
		case common.QuestionNumber, common.QuestionText:
			answer, err = p.askTyped(question)
		default:
			return nil, fmt.Errorf("cannot ask for %s, the terminal does not ask %s questions", question.Key, question.Kind)
		}
		if err != nil {
			return nil, err
		}
		answers[question.Key] = answer
	}
	return answers, nil
}

// makeRaw switches the terminal of input to raw mode and returns the function
// that restores it. SIGINT and SIGTERM restore the terminal before drg exits.
// An input that is not a terminal is left as it is.
func makeRaw(input io.Reader) (restore func(), err error) {
	file, ok := input.(*os.File)
	if !ok || !common.IsTerminal(file) {
		return func() {}, nil
	}
	fd := int(file.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("could not switch the terminal to raw mode: %w", err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case received := <-signals:
			_ = term.Restore(fd, state)
			os.Exit(signalExitBase + int(received.(syscall.Signal)))
		case <-done:
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			signal.Stop(signals)
			close(done)
			_ = term.Restore(fd, state)
		})
	}, nil
}

// askSelect draws the options of question and moves the current one with the
// keys until one is picked. It then replaces the list with one line holding
// the label and the picked value.
func (p *prompter) askSelect(question common.Question) (string, error) {
	restore, err := makeRaw(p.input)
	if err != nil {
		return "", err
	}
	defer restore()

	keys, output, palette := p.keys, p.output, p.palette
	current := 0
	for index, option := range question.Options {
		if option.Value == question.Default {
			current = index
		}
	}
	lineCount := len(question.Options) + 1
	if _, err := io.WriteString(output, renderSelect(palette, question, current)); err != nil {
		return "", err
	}

	for {
		key, err := keys.next()
		if err == io.EOF {
			return "", fmt.Errorf("input ended before an answer for %s was picked", question.Key)
		}
		if err != nil {
			return "", fmt.Errorf("could not read the answer for %s: %w", question.Key, err)
		}

		next := current
		switch key {
		case keyUp:
			next = max(current-1, 0)
		case keyDown:
			next = min(current+1, len(question.Options)-1)
		case keyPick:
			picked := question.Options[current].Value
			if _, err := io.WriteString(output, eraseLines(lineCount)+renderAnswer(palette, question.Label, picked, selectorLineBreak)); err != nil {
				return "", err
			}
			return picked, nil
		case keyCancel:
			if _, err := io.WriteString(output, eraseLines(lineCount)); err != nil {
				return "", err
			}
			return "", errCancelled
		}
		if next == current {
			continue
		}
		current = next
		if _, err := io.WriteString(output, eraseLines(lineCount)+renderSelect(palette, question, current)); err != nil {
			return "", err
		}
	}
}

// askTyped reads one line as the answer to question until question validates
// it. An empty line takes the default. A refused answer prints the error
// under the line and asks again. It then replaces the lines it printed with
// one line holding the label and the answer.
func (p *prompter) askTyped(question common.Question) (string, error) {
	interrupts, stop := p.interrupts()
	defer stop()

	shownDefault := question.DefaultLabel
	if shownDefault == "" {
		shownDefault = question.Default
	}
	prompt := p.palette.Paint(theme.Stdout, theme.RoleHeading, question.Label) + " " +
		p.palette.Paint(theme.Stdout, theme.RoleMuted, "["+shownDefault+"]") + ": "
	// Every refused answer leaves its prompt line and its error line.
	refusalCount := 0
	for {
		if _, err := io.WriteString(p.output, prompt); err != nil {
			return "", err
		}
		line, err := p.readLine(interrupts)
		if errors.Is(err, errCancelled) {
			if _, err := io.WriteString(p.output, lineBreak+eraseLines(refusalCount*2+1)); err != nil {
				return "", err
			}
			return "", err
		}
		if err == io.EOF {
			return "", fmt.Errorf("input ended before an answer for %s was given", question.Key)
		}
		if err != nil {
			return "", fmt.Errorf("could not read the answer for %s: %w", question.Key, err)
		}

		answer := strings.TrimSpace(line)
		if answer == "" {
			answer = question.Default
		}
		if question.Validate != nil {
			if err := question.Validate(answer); err != nil {
				refusal := p.palette.Paint(theme.Stdout, theme.RoleWarning, warningGlyph) + " " + err.Error() + lineBreak
				if _, err := io.WriteString(p.output, refusal); err != nil {
					return "", err
				}
				refusalCount++
				continue
			}
		}
		shownAnswer := answer
		if shownAnswer == "" {
			shownAnswer = shownDefault
		}
		if _, err := io.WriteString(p.output, eraseLines(refusalCount*2+1)+renderAnswer(p.palette, question.Label, shownAnswer, lineBreak)); err != nil {
			return "", err
		}
		return answer, nil
	}
}

// readLine returns the next line of input, or the cancelled error when Ctrl-C
// arrives on interrupts first. A cancelled read leaves its goroutine blocked
// on input, and drg exits right after a cancelled question.
func (p *prompter) readLine(interrupts <-chan os.Signal) (string, error) {
	type readResult struct {
		line string
		err  error
	}
	results := make(chan readResult, 1)
	go func() {
		line, err := p.reader.ReadString(lineFeed)
		results <- readResult{line: line, err: err}
	}()
	select {
	case result := <-results:
		return result.line, result.err
	case <-interrupts:
		return "", errCancelled
	}
}

// renderAnswer renders the line a question leaves once it is answered.
func renderAnswer(palette *theme.Theme, label string, answer string, lineEnd string) string {
	return palette.Paint(theme.Stdout, theme.RoleHeading, label+":") + " " + answer + lineEnd
}

// renderSelect renders the label and the options of question, one per line,
// with the cursor left at the start of the line after them.
func renderSelect(palette *theme.Theme, question common.Question, current int) string {
	var builder strings.Builder
	builder.WriteString(palette.Paint(theme.Stdout, theme.RoleHeading, question.Label) + selectorLineBreak)
	for index, option := range question.Options {
		line := otherOptionMarker + option.Label
		if index == current {
			line = palette.Paint(theme.Stdout, theme.RolePrimary, currentOptionMarker+option.Label)
		}
		if option.Value == question.Default {
			line += " " + palette.Paint(theme.Stdout, theme.RoleMuted, defaultOptionTag)
		}
		builder.WriteString(line + selectorLineBreak)
	}
	return builder.String()
}

// eraseLines moves the cursor up count lines and clears everything below it.
func eraseLines(count int) string {
	return "\r" + fmt.Sprintf(cursorUpFormat, count) + clearToScreenEnd
}

// keyReader reads the keys of the selector from input.
type keyReader struct {
	input   io.Reader
	pending []selectorKey
}

// next returns the next key the selector acts on, skipping every other key.
func (r *keyReader) next() (selectorKey, error) {
	buffer := make([]byte, keyReadSize)
	for len(r.pending) == 0 {
		count, err := r.input.Read(buffer)
		r.pending = parseKeys(buffer[:count])
		if len(r.pending) == 0 && err != nil {
			return 0, err
		}
	}
	key := r.pending[0]
	r.pending = r.pending[1:]
	return key, nil
}

// parseKeys returns the keys in one read from the terminal. A terminal sends
// an escape sequence in a single write, so an escape byte that ends the read
// or is not followed by [ or O is a bare Esc.
func parseKeys(chunk []byte) []selectorKey {
	var keys []selectorKey
	for index := 0; index < len(chunk); index++ {
		switch chunk[index] {
		case upLetter:
			keys = append(keys, keyUp)
		case downLetter:
			keys = append(keys, keyDown)
		case carriageReturn, lineFeed:
			keys = append(keys, keyPick)
		case ctrlCByte:
			keys = append(keys, keyCancel)
		case escapeByte:
			if index+1 >= len(chunk) || (chunk[index+1] != escapeCSI && chunk[index+1] != escapeSS3) {
				keys = append(keys, keyCancel)
				continue
			}
			index++
			for index+1 < len(chunk) {
				index++
				final := chunk[index]
				if final >= escapeFinalFirst && final <= escapeFinalLast {
					if key, ok := arrowKeys[final]; ok {
						keys = append(keys, key)
					}
					break
				}
			}
		}
	}
	return keys
}
