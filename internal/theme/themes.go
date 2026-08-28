package theme

var (
	nordPalette = map[string]string{
		"primary":   "#88C0D0",
		"heading":   "#81A1C1",
		"success":   "#A3BE8C",
		"error":     "#BF616A",
		"warning":   "#EBCB8B",
		"info":      "#88C0D0",
		"muted":     "#4C566A",
		"secondary": "#D8DEE9",
		"border":    "#434C5E",
		"path":      "#A3BE8C",
	}

	monokaiPalette = map[string]string{
		"primary":   "#F92672",
		"heading":   "#F92672",
		"success":   "#A6E22E",
		"error":     "#F92672",
		"warning":   "#FD971F",
		"info":      "#66D9EF",
		"muted":     "#75715E",
		"secondary": "#75715E",
		"border":    "#49483E",
		"path":      "#A6E22E",
	}

	catppuccinMochaPalette = map[string]string{
		"primary":   "#B4BEFE",
		"heading":   "#D6E5FF",
		"success":   "#A6D189",
		"error":     "#E78284",
		"warning":   "#E5C890",
		"info":      "#8CCDFF",
		"muted":     "#626880",
		"secondary": "#A5ADCE",
		"border":    "#585B70",
		"path":      "#A6D189",
	}

	draculaPalette = map[string]string{
		"primary":   "#BD93F9",
		"heading":   "#FF79C6",
		"success":   "#50FA7B",
		"error":     "#FF5555",
		"warning":   "#FFB86C",
		"info":      "#8BE9FD",
		"muted":     "#6272A4",
		"secondary": "#6F8ABF",
		"border":    "#6272A4",
		"path":      "#50FA7B",
	}
)

var bundledPalettes = map[string]map[string]string{
	"nord":             nordPalette,
	"monokai":          monokaiPalette,
	"catppuccin-mocha": catppuccinMochaPalette,
	"dracula":          draculaPalette,
}
