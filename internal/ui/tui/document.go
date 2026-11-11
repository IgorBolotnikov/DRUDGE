package tui

// schemaRef is the $schema reference of the TUI config file, a path relative
// to the file.
const schemaRef = "./schema/tui.json"

// DefaultDocument is the TUI config file drg setup writes when there is none.
type DefaultDocument struct {
	Schema string        `json:"$schema"`
	Tabs   []TabDocument `json:"tabs"`
}

// TabDocument is one Tab of DefaultDocument.
type TabDocument struct {
	Title  string       `json:"title"`
	Layout NodeDocument `json:"layout"`
}

// NodeDocument is one node of the layout of a TabDocument. It leaves out the
// keys a node does not set.
type NodeDocument struct {
	Arrangement Arrangement    `json:"arrangement,omitempty"`
	Children    []NodeDocument `json:"children,omitempty"`
	View        string         `json:"view,omitempty"`
	Title       string         `json:"title,omitempty"`
	Weight      float64        `json:"weight,omitempty"`
	ShowStatus  *bool          `json:"showStatus,omitempty"`
	ViewOptions map[string]any `json:"viewOptions,omitempty"`
}

// NewDefaultDocument returns the TUI config file that holds the Tabs the TUI
// shows when there is no TUI config file.
func NewDefaultDocument() DefaultDocument {
	config := defaultConfig()
	tabs := make([]TabDocument, 0, len(config.Tabs))
	for _, tab := range config.Tabs {
		tabs = append(tabs, TabDocument{Title: tab.Title, Layout: newNodeDocument(tab.Layout)})
	}
	return DefaultDocument{Schema: schemaRef, Tabs: tabs}
}

func newNodeDocument(node NodeConfig) NodeDocument {
	var children []NodeDocument
	for _, child := range node.Children {
		children = append(children, newNodeDocument(child))
	}
	return NodeDocument{
		Arrangement: node.Arrangement,
		Children:    children,
		View:        node.View,
		Title:       node.Title,
		Weight:      node.Weight,
		ShowStatus:  node.ShowStatus,
		ViewOptions: node.ViewOptions,
	}
}
