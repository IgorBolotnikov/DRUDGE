package styles

import (
	"testing"

	glamourstyles "charm.land/glamour/v2/styles"

	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func TestNew_Markdown(t *testing.T) {
	cases := []struct {
		name          string
		themeName     string
		wantHeading   string
		wantH6        string
		wantCode      string
		wantCodeBack  string
		wantCodeTheme string
	}{
		{name: "nord", themeName: "nord", wantHeading: "#81A1C1", wantH6: "#4C566A", wantCode: "#A3BE8C", wantCodeBack: "#3B4252", wantCodeTheme: "nord"},
		{name: "dracula", themeName: "dracula", wantHeading: "#FF79C6", wantH6: "#6272A4", wantCode: "#50FA7B", wantCodeBack: "#44475A", wantCodeTheme: "dracula"},
		{name: "system uses ANSI colors and no chroma style", themeName: "system", wantHeading: "4", wantH6: dimColor, wantCode: "2", wantCodeBack: "8", wantCodeTheme: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			markdown := New(theme.NewTheme(testCase.themeName)).Markdown
			assertColor(t, "heading", markdown.Heading.Color, testCase.wantHeading)
			assertColor(t, "h6", markdown.H6.Color, testCase.wantH6)
			assertColor(t, "inline code", markdown.Code.Color, testCase.wantCode)
			assertColor(t, "inline code background", markdown.Code.BackgroundColor, testCase.wantCodeBack)
			if markdown.CodeBlock.Theme != testCase.wantCodeTheme {
				t.Errorf("code block theme = %q, want %q", markdown.CodeBlock.Theme, testCase.wantCodeTheme)
			}
			if markdown.CodeBlock.Chroma != nil {
				t.Error("code block chroma colors should be unset")
			}
			if markdown.Document.Margin == nil || *markdown.Document.Margin != 0 {
				t.Errorf("document margin = %v, want 0", markdown.Document.Margin)
			}
		})
	}
}

func TestNew_MarkdownLeavesTheDarkStyleAlone(t *testing.T) {
	dark := glamourstyles.DarkStyleConfig
	wantHeading, wantMargin := *dark.Heading.Color, *dark.Document.Margin
	New(theme.NewTheme("nord"))
	assertColor(t, "dark heading", dark.Heading.Color, wantHeading)
	if *dark.Document.Margin != wantMargin {
		t.Errorf("dark document margin = %d, want %d", *dark.Document.Margin, wantMargin)
	}
}

func assertColor(t *testing.T, element string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s color is unset, want %q", element, want)
		return
	}
	if *got != want {
		t.Errorf("%s color = %q, want %q", element, *got, want)
	}
}
