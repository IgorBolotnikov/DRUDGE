package cells

import "testing"

func TestFit(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{name: "pads short text", text: "ab", width: 4, want: "ab  "},
		{name: "keeps text of the exact width", text: "abcd", width: 4, want: "abcd"},
		{name: "cuts long text", text: "abcdef", width: 4, want: "abcd"},
		{name: "counts a wide character as two cells", text: "日本語", width: 5, want: "日本 "},
		{name: "zero width", text: "abc", width: 0, want: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := Fit(testCase.text, testCase.width); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestFitEllipsis(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{name: "pads short text", text: "ab", width: 4, want: "ab  "},
		{name: "keeps text of the exact width", text: "abcd", width: 4, want: "abcd"},
		{name: "ends cut text with an ellipsis", text: "abcdef", width: 4, want: "abc…"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := FitEllipsis(testCase.text, testCase.width); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestBlankLines(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
		want   string
	}{
		{name: "lines of spaces", width: 3, height: 2, want: "   \n   "},
		{name: "zero height", width: 3, height: 0, want: ""},
		{name: "zero width", width: 0, height: 2, want: "\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := BlankLines(testCase.width, testCase.height); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
