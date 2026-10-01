package common

import (
	"fmt"
	"os"
)

// Logger is a simple printf-style logger.
//
// Logger has only two levels: Error and Info.
// It does not need any other levels.
//
// The logis is:
//   - this is an error (breaks the program) -> Error level
//   - everything else (the program still works) -> Info level
type Logger struct {
	prefix     string
	errorLabel string
}

const plainErrorLabel = "Error:"

// NewLogger creates a logger with an optional prefix printed before each
// message. Error prints errorLabel before its message, or a plain label when
// errorLabel is empty.
func NewLogger(prefix string, errorLabel string) *Logger {
	if errorLabel == "" {
		errorLabel = plainErrorLabel
	}
	return &Logger{prefix: prefix, errorLabel: errorLabel}
}

// Info prints an info-level message.
func (l *Logger) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if l.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
	}
	fmt.Printf("%s\n", msg)
}

// Error prints an error-level message to stderr. The top of the program calls
// it with the error a command returned, and the process exits after it.
func (l *Logger) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if l.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", l.errorLabel, msg)
}
