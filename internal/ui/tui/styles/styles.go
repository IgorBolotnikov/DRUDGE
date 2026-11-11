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
	// FocusBorder is the border of the focused Tile and of an open overlay.
	FocusBorder lipgloss.Style
	TileTitle   lipgloss.Style
	Header      lipgloss.Style
	ShortID     lipgloss.Style
	Selection   lipgloss.Style
	Muted       lipgloss.Style
	Error       lipgloss.Style
	Warning     lipgloss.Style
	Statuses    map[task.TaskStatus]lipgloss.Style
	// HealthLabels holds the style of each Drudger health label.
	HealthLabels map[string]lipgloss.Style
	Help         help.Styles
	HelpBox      lipgloss.Style
}

// New builds the styles of the screen from the roles of palette.
func New(palette *theme.Theme) *Styles {
	muted := roleStyle(palette, theme.RoleMuted)
	primary := roleStyle(palette, theme.RolePrimary)
	statuses := make(map[task.TaskStatus]lipgloss.Style, len(style.StatusRoles))
	for status, role := range style.StatusRoles {
		statuses[status] = roleStyle(palette, role)
	}
	healthLabels := make(map[string]lipgloss.Style, len(style.HealthLabelRoles))
	for label, role := range style.HealthLabelRoles {
		healthLabels[label] = roleStyle(palette, role)
	}
	return &Styles{
		Tab:          muted,
		ActiveTab:    primary.Bold(true).Reverse(true),
		Border:       muted,
		FocusBorder:  primary,
		TileTitle:    primary.Bold(true),
		Header:       lipgloss.NewStyle().Bold(true),
		ShortID:      lipgloss.NewStyle().Bold(true),
		Selection:    roleBackgroundStyle(palette, theme.RoleSelection),
		Muted:        muted,
		Error:        roleStyle(palette, theme.RoleError),
		Warning:      roleStyle(palette, theme.RoleWarning),
		Statuses:     statuses,
		HealthLabels: healthLabels,
		Help: help.Styles{
			ShortKey:       lipgloss.NewStyle().Bold(true),
			ShortDesc:      muted,
			ShortSeparator: muted,
			FullKey:        lipgloss.NewStyle().Bold(true),
			FullDesc:       muted,
			FullSeparator:  muted,
			Ellipsis:       muted,
		},
		HelpBox: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(primary.GetForeground()).Padding(0, 1),
	}
}

// Status returns the style of a task status. A status with no role is plain.
func (styles *Styles) Status(status task.TaskStatus) lipgloss.Style {
	if statusStyle, ok := styles.Statuses[status]; ok {
		return statusStyle
	}
	return lipgloss.NewStyle()
}

// HealthLabel returns the style of a Drudger health label. A label with no
// role is plain.
func (styles *Styles) HealthLabel(label string) lipgloss.Style {
	if labelStyle, ok := styles.HealthLabels[label]; ok {
		return labelStyle
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

// roleBackgroundStyle returns a style with the color of role as the
// background and no foreground. A dim or unknown role is plain.
func roleBackgroundStyle(palette *theme.Theme, role string) lipgloss.Style {
	plain := lipgloss.NewStyle()
	roleColor, ok := palette.RoleColor(role)
	if !ok {
		return plain
	}
	switch roleColor.Kind {
	case theme.ColorHex:
		return plain.Background(lipgloss.Color(roleColor.Hex))
	case theme.ColorANSI:
		return plain.Background(lipgloss.ANSIColor(roleColor.ANSI))
	default:
		return plain
	}
}
