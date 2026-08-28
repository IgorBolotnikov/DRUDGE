package theme

// schemaJSON is the bundled JSON schema for theme.json.
const schemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "DRUDGE Theme Configuration",
  "description": "Configures the terminal color theme and optional role overrides for DRUDGE.",
  "type": "object",
  "properties": {
		"$schema": {
			"type": "string",
			"description": "JSON schema reference for configuration validation"
		},
    "theme": {
      "description": "The color theme to use.",
      "type": "string",
      "enum": ["nord", "monokai", "catppuccin-mocha", "dracula"],
      "default": "nord"
    },
    "overrides": {
      "description": "Optional per-role color overrides. Each value must be a 24-bit hex color in \"#rrggbb\" format. Unknown roles are silently ignored.",
      "type": "object",
      "additionalProperties": {
        "type": "string",
        "pattern": "^#[0-9a-fA-F]{6}$",
        "examples": ["#FF5555"]
      },
      "properties": {
        "primary": {
          "description": "Default text / main content color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#88C0D0"]
        },
        "heading": {
          "description": "Heading text color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#81A1C1"]
        },
        "success": {
          "description": "Success message color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#A3BE8C"]
        },
        "error": {
          "description": "Error message color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#BF616A"]
        },
        "warning": {
          "description": "Warning message color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#EBCB8B"]
        },
        "info": {
          "description": "Informational message color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#88C0D0"]
        },
        "muted": {
          "description": "Muted / dimmed text color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#4C566A"]
        },
        "secondary": {
          "description": "Secondary text color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#D8DEE9"]
        },
        "border": {
          "description": "Border / separator color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#434C5E"]
        },
        "path": {
          "description": "File path display color.",
          "type": "string",
          "pattern": "^#[0-9a-fA-F]{6}$",
          "examples": ["#A3BE8C"]
        }
      }
    }
  },
  "additionalProperties": false
}`

// Schema returns the bundled JSON schema for theme.json.
func Schema() []byte {
	return []byte(schemaJSON)
}
