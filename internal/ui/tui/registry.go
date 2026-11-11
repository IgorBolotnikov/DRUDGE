package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds/drudgercheck"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds/drudgers"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds/sessions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds/tasks"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/drudgerlist"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/taskboard"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/taskdetail"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

// optionValueSeparator joins the allowed values of an option in an error.
const optionValueSeparator = ", "

// viewKind is one kind of View a Tile can show, with its options and the keys
// the help lists for it. Every call of its constructor returns a new View, so
// each Tile keeps its own cursor and scroll.
type viewKind struct {
	name        string
	description string
	options     []views.Option
	keys        []key.Binding
	newView     func(viewStyles *styles.Styles, options views.OptionValues) views.View
}

// viewKinds lists every View a Tile can show, sorted by name.
var viewKinds = []viewKind{
	{
		name:        drudgerlist.Name,
		description: drudgerlist.Description,
		keys:        drudgerlist.Keys,
		newView: func(viewStyles *styles.Styles, options views.OptionValues) views.View {
			return drudgerlist.New(viewStyles)
		},
	},
	{
		name:        taskboard.Name,
		description: taskboard.Description,
		keys:        taskboard.Keys,
		newView: func(viewStyles *styles.Styles, options views.OptionValues) views.View {
			return taskboard.New(viewStyles)
		},
	},
	{
		name:        taskdetail.Name,
		description: taskdetail.Description,
		keys:        taskdetail.Keys,
		newView: func(viewStyles *styles.Styles, options views.OptionValues) views.View {
			return taskdetail.New(viewStyles)
		},
	},
	{
		name:        tasklist.Name,
		description: tasklist.Description,
		options:     tasklist.Options,
		keys:        tasklist.Keys,
		newView: func(viewStyles *styles.Styles, options views.OptionValues) views.View {
			return tasklist.New(viewStyles, task.TaskStatus(options[tasklist.StatusOption]))
		},
	},
}

// feedKinds lists every kind of feed a View can subscribe to. The bottom bar
// shows the errors of the feeds in this order.
var feedKinds = []feeds.Kind{
	tasks.Kind,
	drudgers.Kind,
	drudgercheck.Kind,
	sessions.Kind,
}

// findFeedKind returns the kind of feed called name. ok is false for a name
// no feed has.
func findFeedKind(name views.FeedName) (kind feeds.Kind, ok bool) {
	for _, kind := range feedKinds {
		if kind.Name == name {
			return kind, true
		}
	}
	return feeds.Kind{}, false
}

// feedKindNames returns the names of every kind of feed.
func feedKindNames() []string {
	names := make([]string, 0, len(feedKinds))
	for _, kind := range feedKinds {
		names = append(names, string(kind.Name))
	}
	return names
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

// optionValues checks the view options of a Tile against the options of the
// View and returns the value of every option. It refuses an option the View
// does not have, a value of the wrong type and a value outside the allowed
// values. An error starts with path, the path of the view options.
func (kind viewKind) optionValues(rawOptions map[string]any, path string) (views.OptionValues, error) {
	values := kind.defaultOptionValues()
	for _, name := range slices.Sorted(maps.Keys(rawOptions)) {
		optionPath := joinKeyPath(path, name)
		option, ok := kind.findOption(name)
		if !ok {
			return nil, fmt.Errorf("%s: view %q has no option %q, %s", optionPath, kind.name, name, kind.optionHint())
		}
		value, err := checkOptionValue(option, rawOptions[name], optionPath)
		if err != nil {
			return nil, err
		}
		values[name] = value
	}
	return values, nil
}

// defaultOptionValues returns the default value of every option of the View.
func (kind viewKind) defaultOptionValues() views.OptionValues {
	values := make(views.OptionValues, len(kind.options))
	for _, option := range kind.options {
		values[option.Name] = option.Default
	}
	return values
}

func (kind viewKind) findOption(name string) (option views.Option, ok bool) {
	for _, option := range kind.options {
		if option.Name == name {
			return option, true
		}
	}
	return views.Option{}, false
}

// optionHint tells what options the View takes.
func (kind viewKind) optionHint() string {
	if len(kind.options) == 0 {
		return "it takes no options, remove viewOptions"
	}
	names := make([]string, 0, len(kind.options))
	for _, option := range kind.options {
		names = append(names, option.Name)
	}
	return "use one of: " + strings.Join(names, optionValueSeparator)
}

// checkOptionValue returns a decoded JSON value as the value of option. It
// refuses a value of another type than the option takes and a value outside
// its allowed values.
func checkOptionValue(option views.Option, rawValue any, path string) (string, error) {
	switch option.Type {
	case views.OptionString:
		value, ok := rawValue.(string)
		if !ok {
			return "", fmt.Errorf("%s: expected %s, got %s", path, jsonString, goValueKind(rawValue))
		}
		if len(option.Allowed) > 0 && !slices.Contains(option.Allowed, value) {
			return "", fmt.Errorf("%s: %q is not allowed, use one of: %s", path, value, strings.Join(option.Allowed, optionValueSeparator))
		}
		return value, nil
	default:
		return "", fmt.Errorf("%s: option %q has the unknown type %q", path, option.Name, option.Type)
	}
}

// goValueKind names the kind of a value decoded from JSON, as the errors name
// it.
func goValueKind(value any) string {
	switch value.(type) {
	case nil:
		return jsonNull
	case string:
		return jsonString
	case bool:
		return jsonBoolean
	case float64:
		return jsonNumber
	case []any:
		return jsonArray
	default:
		return jsonObject
	}
}
