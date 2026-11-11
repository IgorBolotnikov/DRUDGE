package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// schemaTemplate is the hand-written base of the JSON schema of the TUI config
// file. The interval of each feed, the description and the enum of the view
// key and the view options of each View fill its verbs.
const schemaTemplate = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "DRUDGE TUI Configuration",
  "description": "Configures the tabs of drg tui, the layout of each tab, how often drg tui runs each feed and whether it captures the mouse.",
  "type": "object",
  "properties": {
    "$schema": {
      "type": "string",
      "description": "JSON schema reference for configuration validation"
    },
    "tabs": {
      "description": "The tabs in the order of the top bar. Keys 1 to 9 go to the first nine.",
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["title", "layout"],
        "properties": {
          "title": {
            "description": "The title of the tab in the top bar. Each tab has its own title.",
            "type": "string",
            "minLength": 1
          },
          "layout": {
            "description": "The node that fills the tab: a tile or a split.",
            "$ref": "#/$defs/node"
          }
        },
        "additionalProperties": false
      }
    },
    "refreshSeconds": %s,
    "captureMouse": {
      "description": "Whether drg tui captures the mouse. A click focuses a tile, switches tabs or moves the cursor, and the wheel scrolls the tile under the pointer. When it is false, the terminal keeps its native text selection.",
      "type": "boolean",
      "default": true
    }
  },
  "additionalProperties": false,
  "$defs": {
    "node": {
      "type": "object",
      "description": "A tile or a split. A tile sets view. A split sets arrangement and children.",
      "properties": {
        "arrangement": {
          "description": "How the split places its children.",
          "type": "string",
          "enum": ["side-by-side", "stacked"]
        },
        "children": {
          "description": "The nodes of the split. Each child has the keys of a layout.",
          "type": "array",
          "minItems": 2,
          "items": {"$ref": "#/$defs/node"}
        },
        "view": {
          "description": %s,
          "type": "string",
          "enum": %s
        },
        "title": {
          "description": "The title of the tile in its top border. Only a tile has a title.",
          "type": "string"
        },
        "weight": {
          "description": "The share of the space of its split the node takes, relative to the other children of the split. It may be a fraction.",
          "type": "number",
          "exclusiveMinimum": 0,
          "default": 1
        },
        "showStatus": {
          "description": "Whether the tile shows the status of its view in its bottom border. Only a tile has a status.",
          "type": "boolean",
          "default": true
        },
        "viewOptions": {
          "description": "The options of the view of the tile. Each view takes its own options. Only a tile has view options.",
          "type": "object"
        }
      },
      "additionalProperties": false,
      "allOf": %s
    }
  }
}`

// Indents of the generated parts at their place in the schema template.
const (
	refreshSecondsIndentPrefix = "    "
	schemaIndentPrefix         = "      "
	schemaIndent               = "  "
)

// Schema returns the bundled JSON schema of the TUI config file. The feeds it
// sets intervals for come from the feed registry. The views it allows and the
// view options of each View come from the View registry.
func Schema() []byte {
	refreshSeconds, err := json.MarshalIndent(refreshSecondsSchema(), refreshSecondsIndentPrefix, schemaIndent)
	if err != nil {
		panic(fmt.Sprintf("could not encode the feed intervals: %v", err))
	}
	descriptions := []string{"What the tile shows."}
	for _, kind := range viewKinds {
		descriptions = append(descriptions, fmt.Sprintf("`%s`: %s", kind.name, kind.description))
	}
	viewDescription, err := json.Marshal(strings.Join(descriptions, " "))
	if err != nil {
		panic(fmt.Sprintf("could not encode the view description: %v", err))
	}
	viewNames, err := json.Marshal(viewKindNames())
	if err != nil {
		panic(fmt.Sprintf("could not encode the view names: %v", err))
	}
	viewOptions, err := json.MarshalIndent(viewOptionsSchema(), schemaIndentPrefix, schemaIndent)
	if err != nil {
		panic(fmt.Sprintf("could not encode the view options: %v", err))
	}
	return fmt.Appendf(nil, schemaTemplate, refreshSeconds, viewDescription, viewNames, viewOptions)
}

// schemaNode is the part of JSON schema the view options of a View need.
type schemaNode struct {
	Description          string                `json:"description,omitempty"`
	Type                 string                `json:"type,omitempty"`
	Const                string                `json:"const,omitempty"`
	Enum                 []string              `json:"enum,omitempty"`
	Default              string                `json:"default,omitempty"`
	Required             []string              `json:"required,omitempty"`
	Properties           map[string]schemaNode `json:"properties,omitempty"`
	AdditionalProperties *bool                 `json:"additionalProperties,omitempty"`
}

// schemaCondition applies Then to a node that matches If.
type schemaCondition struct {
	If   schemaNode `json:"if"`
	Then schemaNode `json:"then"`
}

// JSON schema types of the generated parts.
const (
	schemaObjectType = "object"
	schemaNumberType = "number"
)

// viewOptionsSchema returns one condition per View. Each condition gives a
// node whose view is that View the options of the View and refuses any other.
func viewOptionsSchema() []schemaCondition {
	conditions := make([]schemaCondition, 0, len(viewKinds))
	for _, kind := range viewKinds {
		properties := make(map[string]schemaNode, len(kind.options))
		for _, option := range kind.options {
			properties[option.Name] = schemaNode{
				Description: option.Description,
				Type:        string(option.Type),
				Enum:        option.Allowed,
				Default:     option.Default,
			}
		}
		conditions = append(conditions, schemaCondition{
			If: schemaNode{
				Properties: map[string]schemaNode{viewKey: {Const: kind.name}},
				Required:   []string{viewKey},
			},
			Then: schemaNode{
				Properties: map[string]schemaNode{viewOptionsKey: {
					Type:                 schemaObjectType,
					Properties:           properties,
					AdditionalProperties: new(false),
				}},
			},
		})
	}
	return conditions
}

// refreshSecondsNode is the part of JSON schema the feed intervals need. It
// lists the feeds in the order of the feed registry.
type refreshSecondsNode struct {
	Description          string            `json:"description"`
	Type                 string            `json:"type"`
	Properties           feedIntervalNodes `json:"properties"`
	AdditionalProperties bool              `json:"additionalProperties"`
}

// feedIntervalNode is the schema of the interval of one feed, in seconds.
type feedIntervalNode struct {
	name        views.FeedName
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Minimum     float64 `json:"minimum"`
	Default     float64 `json:"default"`
}

// feedIntervalNodes encodes as a JSON object that keeps the order of its
// nodes.
type feedIntervalNodes []feedIntervalNode

func (nodes feedIntervalNodes) MarshalJSON() ([]byte, error) {
	encoded := []byte("{")
	for index, node := range nodes {
		if index > 0 {
			encoded = append(encoded, ',')
		}
		name, err := json.Marshal(node.name)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(node)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, name...)
		encoded = append(encoded, ':')
		encoded = append(encoded, value...)
	}
	return append(encoded, '}'), nil
}

// refreshSecondsSchema returns the interval of every feed with its default
// and its minimum.
func refreshSecondsSchema() refreshSecondsNode {
	nodes := make(feedIntervalNodes, 0, len(feedKinds))
	for _, kind := range feedKinds {
		nodes = append(nodes, feedIntervalNode{
			name:        kind.Name,
			Description: kind.Description,
			Type:        schemaNumberType,
			Minimum:     kind.MinInterval.Seconds(),
			Default:     kind.DefaultInterval.Seconds(),
		})
	}
	return refreshSecondsNode{
		Description: "The seconds between two runs of each feed. A tile runs only the feeds of what it shows. A run never starts while the last run of its feed is going. Each value may be a fraction, like 0.5.",
		Type:        schemaObjectType,
		Properties:  nodes,
	}
}
