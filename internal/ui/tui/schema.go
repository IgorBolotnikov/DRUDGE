package tui

import (
	"encoding/json"
	"fmt"
	"strings"
)

// schemaTemplate is the bundled JSON schema of the TUI config file. The enum
// and the description of the view key fill its two verbs.
const schemaTemplate = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "DRUDGE TUI Configuration",
  "description": "Configures the tabs of drg tui and the layout of each tab.",
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
        }
      },
      "additionalProperties": false
    }
  }
}`

// Schema returns the bundled JSON schema of the TUI config file. The views it
// allows come from the View registry.
func Schema() []byte {
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
	return fmt.Appendf(nil, schemaTemplate, viewDescription, viewNames)
}
