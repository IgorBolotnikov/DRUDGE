package theme

import "testing"

func TestShade(t *testing.T) {
	cases := []struct {
		name       string
		noColor    string
		role       string
		valueShift float64
		want       string
	}{
		{name: "no shift keeps the color", role: RoleError, valueShift: 0, want: "\x1b[38;2;191;97;106m"},
		{name: "negative shift darkens the color", role: RoleError, valueShift: -0.25, want: "\x1b[38;2;127;65;71m"},
		{name: "value is clamped at full brightness", role: RoleError, valueShift: 1, want: "\x1b[38;2;255;130;142m"},
		{name: "value is clamped at black", role: RoleError, valueShift: -1, want: "\x1b[38;2;0;0;0m"},
		{name: "unknown role", role: "nope", valueShift: 0.1, want: ""},
		{name: "NO_COLOR set", noColor: "1", role: RoleError, valueShift: 0.1, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(noColorEnv, testCase.noColor)
			theme := NewTheme("nord")

			if got := theme.Shade(testCase.role, testCase.valueShift); got != testCase.want {
				t.Errorf("Shade(%q, %v) = %q, want %q", testCase.role, testCase.valueShift, got, testCase.want)
			}
		})
	}
}
