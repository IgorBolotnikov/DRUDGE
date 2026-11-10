package tui

import (
	"strings"
	"testing"

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
		wantTitle string
	}{
		{name: "whole title", title: "task-list", width: 30, wantTitle: " task-list "},
		{name: "title cut to the edge", title: "task-list", width: 8, wantTitle: " task"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			screenStyles := tuitest.Styles()
			titleOpening, _, _ := strings.Cut(screenStyles.TileTitle.Render("x"), "x")
			tile := NewTile(&sizeView{name: "a"}, testCase.title, screenStyles, 1)
			tile.SetFocused(true)
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
