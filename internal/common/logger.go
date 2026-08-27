package common

import (
	"fmt"
	"os"
)

// Logger is a simple printf-style logger.
type Logger struct {
	prefix string
}

// NewLogger creates a logger with an optional prefix printed before each message.
func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

// Info prints an info-level message.
func (l *Logger) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if l.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
	}
	fmt.Printf("%s\n", msg)
}

// Error prints an error-level message to stderr.
func (l *Logger) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if l.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
}
