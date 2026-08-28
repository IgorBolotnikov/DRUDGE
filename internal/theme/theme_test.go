package theme

import (
	"fmt"
	"regexp"
	"testing"
)

func TestBundledThemesHaveAllRoles(t *testing.T) {
	for name := range bundledPalettes {
		t.Run(name, func(t *testing.T) {
			th := NewTheme(name)
			for _, role := range allRoles() {
				if th.Hex(role) == "" {
					t.Errorf("theme %q missing role %q", name, role)
				}
			}
		})
	}
}

func TestANSI_InvalidRole(t *testing.T) {
	th := NewTheme("nord")
	got := th.ANSI("nonexistent")
	if got != "\x1b[0m" {
		t.Errorf("expected reset for unknown role, got %q", got)
	}
}

func TestANSI_ValidRoleFormat(t *testing.T) {
	th := NewTheme("nord")
	// Match \x1b[38;2;R;G;mb\x1b[0m
	re := regexp.MustCompile(`\x1b\[38;2;\d{1,3};\d{1,3};\d{1,3}m\x1b\[0m`)
	for _, role := range allRoles() {
		got := th.ANSI(role)
		if !re.MatchString(got) {
			t.Errorf("ANSI(%q) = %q does not match expected 24-bit ANSI format", role, got)
		}
	}
}

func TestANSI_ExactSequence(t *testing.T) {
	th := NewTheme("dracula")
	// dracula error is #FF5555 => R=255, G=85, B=85
	got := th.ANSI("error")
	expected := fmt.Sprintf("\x1b[38;2;255;85;85m\x1b[0m")
	if got != expected {
		t.Errorf("ANSI(error) = %q, want %q", got, expected)
	}
}

func TestReset(t *testing.T) {
	th := NewTheme("nord")
	got := th.Reset()
	expected := "\x1b[0m"
	if got != expected {
		t.Errorf("Reset() = %q, want %q", got, expected)
	}
}

func TestHex_ExactValue(t *testing.T) {
	th := NewTheme("dracula")
	tests := []struct {
		role   string
		expect string
	}{
		{"primary", "#BD93F9"},
		{"error", "#FF5555"},
		{"success", "#50FA7B"},
		{"heading", "#FF79C6"},
		{"muted", "#6272A4"},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := th.Hex(tt.role)
			if got != tt.expect {
				t.Errorf("Hex(%q) = %q, want %q", tt.role, got, tt.expect)
			}
		})
	}
}

func TestColors_ReturnsCopy(t *testing.T) {
	th := NewTheme("nord")
	c1 := th.Colors()
	c1["primary"] = "#000000"
	c2 := th.Colors()
	if c2["primary"] == "#000000" {
		t.Error("Colors() should return a copy; modifying the result should not affect the theme")
	}
}

func TestColors_NotSameReference(t *testing.T) {
	th := NewTheme("nord")
	c1 := th.Colors()
	c1["newrole"] = "#123456"
	c2 := th.Colors()
	if _, ok := c2["newrole"]; ok {
		t.Error("Colors() should return a copy; new keys should not leak between calls")
	}
}

func TestNewTheme_UnknownName(t *testing.T) {
	th := NewTheme("nonexistent-theme")
	for _, role := range allRoles() {
		if th.Hex(role) != "" {
			t.Errorf("unknown theme should have empty colors, but Hex(%q) = %q", role, th.Hex(role))
		}
	}
}

func allRoles() []string {
	return []string{
		"primary", "heading", "success", "error", "warning",
		"info", "muted", "secondary", "border", "path",
	}
}
