package config

// schemaJSON is the bundled JSON schema for config.json.
const schemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "DRUDGE Global Configuration",
  "description": "Configures global DRUDGE settings, such as the Drudger environment and harness.",
  "type": "object",
  "properties": {
    "$schema": {
      "type": "string",
      "description": "JSON schema reference for configuration validation"
    },
    "drudger": {
      "description": "Settings for the Drudgers that execute tasks.",
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
        "maxConcurrentDrudgers": {
          "description": "How many Drudgers may work on one project at once.",
          "type": "integer",
          "minimum": 1,
          "default": 3
        },
        "sandboxTimeouts": {
          "description": "How long DRUDGE waits for a sandbox command before it kills it. A command that needs longer than its timeout is killed and reported.",
          "type": "object",
          "properties": {
            "listSeconds": {
              "description": "Seconds allowed for listing the sandboxes.",
              "type": "integer",
              "minimum": 1,
              "default": 30
            },
            "createSeconds": {
              "description": "Seconds allowed for creating a sandbox. A first creation pulls an image, which is why this one is generous.",
              "type": "integer",
              "minimum": 1,
              "default": 600
            },
            "removeSeconds": {
              "description": "Seconds allowed for removing a sandbox.",
              "type": "integer",
              "minimum": 1,
              "default": 120
            }
          },
          "additionalProperties": false
        },
        "gitTimeouts": {
          "description": "How long DRUDGE waits for a git command before it kills it. Git has no timeout of its own.",
          "type": "object",
          "properties": {
            "fetchSeconds": {
              "description": "Seconds allowed for fetching from a remote. Kept short so an unreachable remote costs seconds.",
              "type": "integer",
              "minimum": 1,
              "default": 15
            },
            "worktreeSeconds": {
              "description": "Seconds allowed for creating a worktree, which is a full checkout.",
              "type": "integer",
              "minimum": 1,
              "default": 600
            },
            "commandSeconds": {
              "description": "Seconds allowed for every other git command.",
              "type": "integer",
              "minimum": 1,
              "default": 60
            }
          },
          "additionalProperties": false
        }
      },
      "additionalProperties": false
    },
    "task": {
      "description": "Settings for tasks. A project config may override each of them.",
      "type": "object",
      "properties": {
        "defaultStatus": {
          "description": "Status of a new task created without --status. A draft task waits for review, a todo task is ready to run.",
          "type": "string",
          "enum": ["draft", "todo"],
          "default": "draft"
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

// localSchemaJSON is the bundled JSON schema for the local config file.
const localSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "DRUDGE Project Configuration",
  "description": "Links a directory to a DRUDGE project and overrides global settings for it.",
  "type": "object",
  "properties": {
    "$schema": {
      "type": "string",
      "description": "JSON schema reference for configuration validation"
    },
    "projectSlug": {
      "description": "Slug of the project this directory is linked to.",
      "type": "string"
    },
    "promptFile": {
      "description": "File name of the prompt handed to an agent, resolved under .drudge/prompts/ of the project directory. Omit to use the global prompt file.",
      "type": "string"
    },
    "maxConcurrentDrudgers": {
      "description": "How many Drudgers may work on one project at once.",
      "type": "integer",
      "minimum": 1,
      "default": 3
    },
    "task": {
      "description": "Settings for tasks. Each one overrides the global config.",
      "type": "object",
      "properties": {
        "defaultStatus": {
          "description": "Status of a new task created without --status. A draft task waits for review, a todo task is ready to run.",
          "type": "string",
          "enum": ["draft", "todo"],
          "default": "draft"
        }
      },
      "additionalProperties": false
    },
    "repositories": {
      "description": "The git repositories of the project. drg project init writes the list.",
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "path": {
            "description": "Where the repository sits relative to the project directory. A project directory that is itself a repository records \".\".",
            "type": "string"
          },
          "defaultBranch": {
            "description": "The branch work is cut from. Omit to read it from origin/HEAD.",
            "type": "string"
          }
        },
        "required": ["path"],
        "additionalProperties": false
      }
    }
  },
  "required": ["projectSlug"],
  "additionalProperties": false
}`

// LocalSchema returns the bundled JSON schema for the local config file.
func LocalSchema() []byte {
	return []byte(localSchemaJSON)
}
