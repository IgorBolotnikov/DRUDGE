package cmd

import (
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
)

const (
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

// askQuestions asks every question on output, reads the keys from input and
// returns the answers. The terminal of input is in raw mode while it asks.
func askQuestions(input io.Reader, output io.Writer, palette *theme.Theme, questions []common.Question) (common.Answers, error) {
	answers := make(common.Answers, len(questions))
	if len(questions) == 0 {
		return answers, nil
	}
	restore, err := makeRaw(input)
	if err != nil {
		return nil, err
	}
	defer restore()

	keys := &keyReader{input: input}
	for _, question := range questions {
		if question.Kind != common.QuestionSelect {
			return nil, fmt.Errorf("cannot ask for %s, the terminal only asks %s questions", question.Key, common.QuestionSelect)
		}
		answer, err := askSelect(keys, output, palette, question)
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
func askSelect(keys *keyReader, output io.Writer, palette *theme.Theme, question common.Question) (string, error) {
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
			collapsed := palette.Paint(theme.Stdout, theme.RoleHeading, question.Label+":") + " " + picked + selectorLineBreak
			if _, err := io.WriteString(output, eraseLines(lineCount)+collapsed); err != nil {
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
