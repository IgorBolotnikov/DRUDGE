package common

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func captureError(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	f()
	w.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestLogger_Info_NoPrefix(t *testing.T) {
	l := NewLogger("", Labels{})
	out := captureOutput(func() { l.Info("hello %s", "world") })
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected 'hello world', got %q", out)
	}
}

func TestLogger_Info_WithPrefix(t *testing.T) {
	l := NewLogger("drudge", Labels{})
	out := captureOutput(func() { l.Info("booting up") })
	if !strings.Contains(out, "[drudge] booting up") {
		t.Errorf("expected '[drudge] booting up', got %q", out)
	}
}

func TestLogger_Info_MultipleArgs(t *testing.T) {
	l := NewLogger("", Labels{})
	out := captureOutput(func() { l.Info("%d %s %d", 1, "two", 3) })
	if !strings.Contains(out, "1 two 3") {
		t.Errorf("expected '1 two 3', got %q", out)
	}
}

func TestLogger_Error_Label(t *testing.T) {
	testCases := []struct {
		name       string
		errorLabel string
		want       string
	}{
		{name: "prints the label it was given", errorLabel: "\x1b[31mError:\x1b[0m", want: "\x1b[31mError:\x1b[0m oops did something\n"},
		{name: "prints a plain label when the label is empty", errorLabel: "", want: "Error: oops did something\n"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := NewLogger("", Labels{Error: testCase.errorLabel})
			got := captureError(func() { logger.Error("oops %s", "did something") })
			if got != testCase.want {
				t.Errorf("got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestLogger_Warn_Label(t *testing.T) {
	testCases := []struct {
		name      string
		warnLabel string
		want      string
	}{
		{name: "prints the label it was given", warnLabel: "\x1b[33m!\x1b[0m", want: "\x1b[33m!\x1b[0m could not fetch main\n"},
		{name: "prints a plain label when the label is empty", warnLabel: "", want: "! could not fetch main\n"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := NewLogger("", Labels{Warn: testCase.warnLabel})
			var stdout string
			stderr := captureError(func() {
				stdout = captureOutput(func() { logger.Warn("could not fetch %s", "main") })
			})
			if stderr != testCase.want {
				t.Errorf("stderr = %q, want %q", stderr, testCase.want)
			}
			if stdout != "" {
				t.Errorf("expected nothing on stdout, got %q", stdout)
			}
		})
	}
}

func TestLogger_Error_WithPrefix(t *testing.T) {
	l := NewLogger("drudge", Labels{})
	out := captureError(func() { l.Error("something failed") })
	if !strings.Contains(out, "[drudge] something failed") {
		t.Errorf("expected '[drudge] something failed', got %q", out)
	}
}

func TestLogger_InfoVsError_DifferentStreams(t *testing.T) {
	l := NewLogger("", Labels{})
	infoOut := captureOutput(func() { l.Info("info msg") })
	errOut := captureError(func() { l.Error("err msg") })
	if strings.Contains(infoOut, "Error:") {
		t.Error("Info should not write to stderr")
	}
	if !strings.Contains(errOut, "err msg") {
		t.Errorf("Error should write to stderr, got %q", errOut)
	}
}

func TestLogger_Indented(t *testing.T) {
	testCases := []struct {
		name       string
		print      func(logger *Logger)
		wantStdout string
		wantStderr string
	}{
		{name: "info", print: func(logger *Logger) { logger.Info("hello") }, wantStdout: "  hello\n"},
		{name: "warn puts the indent before the label", print: func(logger *Logger) { logger.Warn("careful") }, wantStderr: "  ! careful\n"},
		{name: "error puts the indent before the label", print: func(logger *Logger) { logger.Error("oops") }, wantStderr: "  Error: oops\n"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := NewLogger("", Labels{}).Indented("  ")
			var stdout string
			stderr := captureError(func() {
				stdout = captureOutput(func() { testCase.print(logger) })
			})
			if stdout != testCase.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, testCase.wantStdout)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, testCase.wantStderr)
			}
		})
	}
}
