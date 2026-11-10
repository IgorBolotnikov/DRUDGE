package tui

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

// viewKind is one kind of View a Tile can show. Every call of its constructor
// returns a new View, so each Tile keeps its own cursor and scroll.
type viewKind struct {
	name        string
	description string
	newView     func(viewStyles *styles.Styles) views.View
}

// viewKinds lists every View a Tile can show, sorted by name.
var viewKinds = []viewKind{
	{
		name:        tasklist.Name,
		description: tasklist.Description,
		newView:     func(viewStyles *styles.Styles) views.View { return tasklist.New(viewStyles) },
	},
}

// findViewKind returns the kind of View called name. ok is false for a name no
// View has.
func findViewKind(name string) (kind viewKind, ok bool) {
	for _, kind := range viewKinds {
		if kind.name == name {
			return kind, true
		}
	}
	return viewKind{}, false
}

// viewKindNames returns the names of every View a Tile can show.
func viewKindNames() []string {
	names := make([]string, 0, len(viewKinds))
	for _, kind := range viewKinds {
		names = append(names, kind.name)
	}
	return names
}
