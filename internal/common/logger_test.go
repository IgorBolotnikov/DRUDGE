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
	l := NewLogger("")
	out := captureOutput(func() { l.Info("hello %s", "world") })
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected 'hello world', got %q", out)
	}
}

func TestLogger_Info_WithPrefix(t *testing.T) {
	l := NewLogger("drudge")
	out := captureOutput(func() { l.Info("booting up") })
	if !strings.Contains(out, "[drudge] booting up") {
		t.Errorf("expected '[drudge] booting up', got %q", out)
	}
}

func TestLogger_Info_MultipleArgs(t *testing.T) {
	l := NewLogger("")
	out := captureOutput(func() { l.Info("%d %s %d", 1, "two", 3) })
	if !strings.Contains(out, "1 two 3") {
		t.Errorf("expected '1 two 3', got %q", out)
	}
}

func TestLogger_Error_NoPrefix(t *testing.T) {
	l := NewLogger("")
	out := captureError(func() { l.Error("oops %s", "did something") })
	if !strings.Contains(out, "Error: oops did something") {
		t.Errorf("expected 'Error: oops did something', got %q", out)
	}
}

func TestLogger_Error_WithPrefix(t *testing.T) {
	l := NewLogger("drudge")
	out := captureError(func() { l.Error("something failed") })
	if !strings.Contains(out, "[drudge] something failed") {
		t.Errorf("expected '[drudge] something failed', got %q", out)
	}
}

func TestLogger_InfoVsError_DifferentStreams(t *testing.T) {
	l := NewLogger("")
	infoOut := captureOutput(func() { l.Info("info msg") })
	errOut := captureError(func() { l.Error("err msg") })
	if strings.Contains(infoOut, "Error:") {
		t.Error("Info should not write to stderr")
	}
	if !strings.Contains(errOut, "err msg") {
		t.Errorf("Error should write to stderr, got %q", errOut)
	}
}
