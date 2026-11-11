package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
)

// styledText returns the text that line shows in the style whose escape
// sequence opens rendered sample text. ok is false when the line has no text
// in that style.
func styledText(line string, styleOpening string) (text string, ok bool) {
	start := strings.Index(line, styleOpening)
	if start < 0 {
		return "", false
	}
	afterOpening := line[start+len(styleOpening):]
	end := strings.Index(afterOpening, "\x1b[")
	if end < 0 {
		return afterOpening, true
	}
	return afterOpening[:end], true
}

func TestTile_RenderTitleStyle(t *testing.T) {
	cases := []struct {
		name      string
		title     string
		width     int
		isZoomed  bool
		wantTitle string
	}{
		{name: "whole title", title: "task-list", width: 30, wantTitle: " ▸ task-list "},
		{name: "zoomed title", title: "task-list", width: 30, isZoomed: true, wantTitle: " ▸ task-list [zoom] "},
		{name: "title cut to the edge", title: "task-list", width: 8, wantTitle: " ▸ ta"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			screenStyles := tuitest.Styles()
			titleOpening, _, _ := strings.Cut(screenStyles.TileTitle.Render("x"), "x")
			tile := NewTile(&sizeView{name: "a"}, testCase.title, screenStyles, 1, true)
			tile.SetFocused(true)
			tile.SetZoomed(testCase.isZoomed)
			tile.SetSize(testCase.width, 5)

			topEdge, _, _ := strings.Cut(tile.Render(), "\n")
			got, ok := styledText(topEdge, titleOpening)
			if !ok {
				t.Fatalf("expected the title style in the top edge %q", topEdge)
			}
			if got != testCase.wantTitle {
				t.Errorf("expected the title style on %q, got it on %q", testCase.wantTitle, got)
			}
		})
	}
}

func TestTile_RenderBorderStyle(t *testing.T) {
	cases := []struct {
		name          string
		isFocused     bool
		wantFocusLine bool
	}{
		{name: "focused tile", isFocused: true, wantFocusLine: true},
		{name: "unfocused tile", isFocused: false, wantFocusLine: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			screenStyles := tuitest.Styles()
			focusOpening, _, _ := strings.Cut(screenStyles.FocusBorder.Render("x"), "x")
			tile := NewTile(&sizeView{name: "a"}, "task-list", screenStyles, 1, true)
			tile.SetFocused(testCase.isFocused)
			tile.SetSize(30, 5)

			lines := strings.Split(tile.Render(), "\n")
			bottomEdge := lines[len(lines)-1]
			got, ok := styledText(bottomEdge, focusOpening)
			hasFocusBorder := ok && strings.HasPrefix(got, "╰")
			if hasFocusBorder != testCase.wantFocusLine {
				t.Errorf("expected the focus border on the bottom edge to be %v, got %v in %q", testCase.wantFocusLine, hasFocusBorder, bottomEdge)
			}
		})
	}
}

func TestTile_RenderBottomEdge(t *testing.T) {
	cases := []struct {
		name          string
		status        string
		width         int
		isStatusShown bool
		want          string
	}{
		{name: "status before the corner", status: "1/5", width: 12, isStatusShown: true, want: "╰──── 1/5 ─╯"},
		{name: "status cut from the start", status: "3f9a1c2e 40%", width: 12, isStatusShown: true, want: "╰1c2e 40% ─╯"},
		{name: "empty status", status: "", width: 12, isStatusShown: true, want: "╰──────────╯"},
		{name: "hidden status", status: "1/5", width: 12, isStatusShown: false, want: "╰──────────╯"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tile := NewTile(&sizeView{name: testCase.status}, "a", tuitest.Styles(), 1, testCase.isStatusShown)
			tile.SetSize(testCase.width, 5)

			lines := strings.Split(tile.Render(), "\n")
			if got := ansi.Strip(lines[len(lines)-1]); got != testCase.want {
				t.Errorf("expected the bottom edge %q, got %q", testCase.want, got)
			}
		})
	}
}
