package config

// schemaJSON is the bundled JSON schema for config.json.
const schemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "DRUDGE Global Configuration",
  "description": "Configures global DRUDGE settings, such as the runner environment and harness.",
  "type": "object",
  "properties": {
    "$schema": {
      "type": "string",
      "description": "JSON schema reference for configuration validation"
    },
    "runner": {
      "description": "Settings for the runner DRUDGE uses to execute tasks.",
      "type": "object",
      "properties": {
        "environment": {
          "description": "The environment tasks run in.",
          "type": "string",
          "enum": ["docker-sbx"],
          "default": "docker-sbx"
        },
        "harness": {
          "description": "The agent harness used to run tasks.",
          "type": "string",
          "enum": ["claude-code", "opencode"],
          "default": "claude-code"
        },
        "promptFile": {
          "description": "File name of the prompt handed to an agent, resolved under ~/.drudge/prompts/. Omit to use the built-in default prompt.",
          "type": "string"
        },
        "maxConcurrentRunners": {
          "description": "How many runners may work on one project at once.",
          "type": "integer",
          "minimum": 1,
          "default": 3
        }
      },
      "additionalProperties": false
    }
  },
  "additionalProperties": false
}`

// Schema returns the bundled JSON schema for config.json.
func Schema() []byte {
	return []byte(schemaJSON)
}
