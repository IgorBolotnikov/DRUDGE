package theme

import "testing"

func TestPaint(t *testing.T) {
	cases := []struct {
		name      string
		noColor   string
		themeName string
		isColorOn map[Stream]bool
		stream    Stream
		role      string
		want      string
	}{
		{name: "color on for stdout", themeName: "nord", stream: Stdout, role: RoleError, want: "\x1b[38;2;191;97;106mtext\x1b[0m"},
		{name: "color on for stderr", themeName: "nord", stream: Stderr, role: RoleError, want: "\x1b[38;2;191;97;106mtext\x1b[0m"},
		{name: "an ANSI color", themeName: systemTheme, stream: Stdout, role: RoleSuccess, want: "\x1b[32mtext\x1b[0m"},
		{name: "color off for stdout only", themeName: "nord", isColorOn: map[Stream]bool{Stderr: true}, stream: Stdout, role: RoleError, want: "text"},
		{name: "color off for stderr only", themeName: "nord", isColorOn: map[Stream]bool{Stdout: true}, stream: Stderr, role: RoleError, want: "text"},
		{name: "unknown role", themeName: "nord", stream: Stdout, role: "nope", want: "text"},
		{name: "NO_COLOR set", noColor: "1", themeName: "nord", stream: Stdout, role: RoleError, want: "text"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(noColorEnv, testCase.noColor)
			t.Setenv(forceColorEnv, "1")
			theme := NewTheme(testCase.themeName)
			if testCase.isColorOn != nil {
				theme.isColorOn = testCase.isColorOn
			}

			if got := theme.Paint(testCase.stream, testCase.role, "text"); got != testCase.want {
				t.Errorf("Paint(%v, %q, \"text\") = %q, want %q", testCase.stream, testCase.role, got, testCase.want)
			}
		})
	}
}

func TestBold(t *testing.T) {
	cases := []struct {
		name      string
		noColor   string
		themeName string
		themeFile string
		isColorOn map[Stream]bool
		stream    Stream
		want      string
	}{
		{name: "color on for stdout", themeName: "nord", stream: Stdout, want: "\x1b[1mtext\x1b[0m"},
		{name: "color on for stderr", themeName: "nord", stream: Stderr, want: "\x1b[1mtext\x1b[0m"},
		{name: "the system theme", themeName: systemTheme, stream: Stdout, want: "\x1b[1mtext\x1b[0m"},
		{name: "theme file overrides do not change bold", themeFile: `{"theme": "nord", "overrides": {"bold": "#ff0000"}}`, stream: Stdout, want: "\x1b[1mtext\x1b[0m"},
		{name: "color off for stdout only", themeName: "nord", isColorOn: map[Stream]bool{Stderr: true}, stream: Stdout, want: "text"},
		{name: "color off for stderr only", themeName: "nord", isColorOn: map[Stream]bool{Stdout: true}, stream: Stderr, want: "text"},
		{name: "NO_COLOR set", noColor: "1", themeName: "nord", stream: Stdout, want: "text"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(noColorEnv, testCase.noColor)
			t.Setenv(forceColorEnv, "1")
			setupTempHome(t, testCase.themeFile)
			theme, err := Load(testCase.themeName)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if testCase.isColorOn != nil {
				theme.isColorOn = testCase.isColorOn
			}

			if got := theme.Bold(testCase.stream, "text"); got != testCase.want {
				t.Errorf("Bold(%v, \"text\") = %q, want %q", testCase.stream, got, testCase.want)
			}
		})
	}
}
