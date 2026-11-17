package styles

import (
	"strconv"

	glamouransi "charm.land/glamour/v2/ansi"
	glamourstyles "charm.land/glamour/v2/styles"

	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

// codeThemes maps each bundled theme with 24-bit colors to the chroma style
// that highlights code blocks in the same palette. Chroma reads only hex
// colors, so the system theme has no entry.
var codeThemes = map[string]string{
	"nord":             "nord",
	"monokai":          "monokai",
	"catppuccin-mocha": "catppuccin-mocha",
	"dracula":          "dracula",
}

// markdown builds the glamour style of a markdown document from the roles of
// palette. It keeps the layout of the dark glamour style and drops the margin
// around the document, which the Tile border already gives. A code block of a
// theme with no chroma style renders in one color.
func markdown(palette *theme.Theme) glamouransi.StyleConfig {
	// The copy shares its pointers with the dark style, so every change below
	// sets a field to a new value and never writes through a pointer.
	style := glamourstyles.DarkStyleConfig
	style.Document.Margin = new(uint(0))
	style.Document.Color = nil

	style.Heading.StylePrimitive = paintText(palette, theme.RoleHeading, style.Heading.StylePrimitive)
	style.H1.StylePrimitive = paintText(palette, theme.RolePrimary, style.H1.StylePrimitive)
	style.H1.BackgroundColor = nil
	style.H1.Inverse = new(true)
	style.H6.StylePrimitive = paintText(palette, theme.RoleMuted, style.H6.StylePrimitive)

	style.HorizontalRule = paintText(palette, theme.RoleBorder, style.HorizontalRule)

	style.Link = paintText(palette, theme.RoleMuted, style.Link)
	style.LinkText = paintText(palette, theme.RoleInfo, style.LinkText)
	style.Image = paintText(palette, theme.RoleMuted, style.Image)
	style.ImageText = paintText(palette, theme.RoleInfo, style.ImageText)

	style.Code.StylePrimitive = paintText(palette, theme.RolePath, style.Code.StylePrimitive)
	style.Code.StylePrimitive = paintBackground(palette, theme.RoleSelection, style.Code.StylePrimitive)
	style.CodeBlock.StylePrimitive = paintText(palette, theme.RoleSecondary, style.CodeBlock.StylePrimitive)
	style.CodeBlock.Theme = codeThemes[palette.Name()]
	style.CodeBlock.Chroma = nil
	return style
}

// paintText returns text in the color of role. An unknown role leaves text as
// it is.
func paintText(palette *theme.Theme, role string, text glamouransi.StylePrimitive) glamouransi.StylePrimitive {
	roleColor, ok := palette.RoleColor(role)
	if !ok {
		return text
	}
	text.Color = glamourColor(roleColor)
	return text
}

// paintBackground returns text on the color of role. An unknown role leaves
// text as it is.
func paintBackground(palette *theme.Theme, role string, text glamouransi.StylePrimitive) glamouransi.StylePrimitive {
	roleColor, ok := palette.RoleColor(role)
	if !ok {
		return text
	}
	text.BackgroundColor = glamourColor(roleColor)
	return text
}

// dimColor is the ANSI color that stands in for dim. Glamour renders no faint
// text, so dim becomes bright black.
const dimColor = "8"

// glamourColor returns a role color as a glamour color: a hex string or the
// number of an ANSI color.
func glamourColor(roleColor theme.RoleColor) *string {
	switch roleColor.Kind {
	case theme.ColorHex:
		return new(roleColor.Hex)
	case theme.ColorANSI:
		return new(strconv.Itoa(roleColor.ANSI))
	case theme.ColorDim:
		return new(dimColor)
	default:
		return nil
	}
}
