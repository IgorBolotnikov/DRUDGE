// Package styles builds the lipgloss styles of the TUI from the theme.
package styles

import (
	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/style"
)

// Styles holds every style of the screen, built once from the theme.
type Styles struct {
	Tab       lipgloss.Style
	ActiveTab lipgloss.Style
	Border    lipgloss.Style
	TileTitle lipgloss.Style
	Header    lipgloss.Style
	ShortID   lipgloss.Style
	Cursor    lipgloss.Style
	Muted     lipgloss.Style
	Error     lipgloss.Style
	Warning   lipgloss.Style
	Statuses  map[task.TaskStatus]lipgloss.Style
	Help      help.Styles
}

// New builds the styles of the screen from the roles of palette.
func New(palette *theme.Theme) *Styles {
	muted := roleStyle(palette, theme.RoleMuted)
	statuses := make(map[task.TaskStatus]lipgloss.Style, len(style.StatusRoles))
	for status, role := range style.StatusRoles {
		statuses[status] = roleStyle(palette, role)
	}
	return &Styles{
		Tab:       muted,
		ActiveTab: roleStyle(palette, theme.RolePrimary).Bold(true).Reverse(true),
		Border:    muted,
		TileTitle: roleStyle(palette, theme.RolePrimary).Bold(true),
		Header:    lipgloss.NewStyle().Bold(true),
		ShortID:   lipgloss.NewStyle().Bold(true),
		Cursor:    lipgloss.NewStyle().Reverse(true),
		Muted:     muted,
		Error:     roleStyle(palette, theme.RoleError),
		Warning:   roleStyle(palette, theme.RoleWarning),
		Statuses:  statuses,
		Help: help.Styles{
			ShortKey:       lipgloss.NewStyle().Bold(true),
			ShortDesc:      muted,
			ShortSeparator: muted,
			Ellipsis:       muted,
		},
	}
}

// Status returns the style of a task status. A status with no role is plain.
func (styles *Styles) Status(status task.TaskStatus) lipgloss.Style {
	if statusStyle, ok := styles.Statuses[status]; ok {
		return statusStyle
	}
	return lipgloss.NewStyle()
}

// roleStyle returns a style in the color of role. A dim role is faint, so the
// system theme keeps the look of the terminal. An unknown role is plain.
func roleStyle(palette *theme.Theme, role string) lipgloss.Style {
	plain := lipgloss.NewStyle()
	roleColor, ok := palette.RoleColor(role)
	if !ok {
		return plain
	}
	switch roleColor.Kind {
	case theme.ColorHex:
		return plain.Foreground(lipgloss.Color(roleColor.Hex))
	case theme.ColorANSI:
		return plain.Foreground(lipgloss.ANSIColor(roleColor.ANSI))
	case theme.ColorDim:
		return plain.Faint(true)
	default:
		return plain
	}
}
