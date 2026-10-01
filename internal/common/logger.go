package common

import (
	"fmt"
	"os"
)

// Logger is a simple printf-style logger with three levels.
//
//   - Error means the command stops.
//   - Warn means something went wrong and the command goes on.
//   - Info is everything else.
type Logger struct {
	prefix string
	labels Labels
}

// Labels are what a Logger prints before the message of an error and of a
// warning.
type Labels struct {
	Error string
	Warn  string
}

const (
	plainErrorLabel = "Error:"
	plainWarnLabel  = "!"
)

// NewLogger creates a logger with an optional prefix printed before each
// message. An empty label falls back to a plain one.
func NewLogger(prefix string, labels Labels) *Logger {
	if labels.Error == "" {
		labels.Error = plainErrorLabel
	}
	if labels.Warn == "" {
		labels.Warn = plainWarnLabel
	}
	return &Logger{prefix: prefix, labels: labels}
}

// Info prints an info-level message.
func (l *Logger) Info(format string, args ...any) {
	fmt.Printf("%s\n", l.format(format, args...))
}

// Warn prints a warn-level message to stderr.
func (l *Logger) Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", l.labels.Warn, l.format(format, args...))
}

// Error prints an error-level message to stderr. The top of the program calls
// it with the error a command returned, and the process exits after it.
func (l *Logger) Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", l.labels.Error, l.format(format, args...))
}

func (l *Logger) format(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	if l.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
	}
	return msg
}
