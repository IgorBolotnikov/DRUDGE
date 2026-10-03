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
        "pageSize": {
          "description": "How many Drudgers drg drudger list shows on one page. 0 shows every Drudger on one page.",
          "type": "integer",
          "minimum": 0,
          "default": 20
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
        },
        "branchFormat": {
          "description": "Format of the names of the branches DRUDGE makes for tasks. Literal text mixes with the placeholders {{taskShortID}}, the leading characters of the task id, and {{taskSlug}}, the task title folded to lowercase words joined by -. A format needs {{taskShortID}} or {{taskSlug}}. Once the placeholders are filled in, repeated / collapse into one, -, _, . and / are trimmed from both ends of every segment and empty segments are dropped. A changed format applies from the next run. Branches made before stay as they are.",
          "type": "string",
          "default": "drudge/{{taskShortID}}-{{taskSlug}}"
        },
        "pageSize": {
          "description": "How many tasks drg task list shows on one page. 0 shows every task on one page.",
          "type": "integer",
          "minimum": 0,
          "default": 20
        }
      },
      "additionalProperties": false
    },
    "project": {
      "description": "Settings for projects.",
      "type": "object",
      "properties": {
        "pageSize": {
          "description": "How many projects drg project list shows on one page. 0 shows every project on one page.",
          "type": "integer",
          "minimum": 0,
          "default": 20
        }
      },
      "additionalProperties": false
    },
    "remote": {
      "description": "Settings for the provider the repositories of a project are hosted on. A project config may override each of them.",
      "type": "object",
      "properties": {
        "provider": {
          "description": "The provider the repositories are hosted on. Required when pull requests are on.",
          "type": "string",
          "enum": ["github"]
        },
        "timeoutSeconds": {
          "description": "Seconds allowed for one call of the provider CLI.",
          "type": "integer",
          "minimum": 1,
          "default": 60
        },
        "pullRequests": {
          "description": "Settings for the pull requests DRUDGE opens.",
          "type": "object",
          "properties": {
            "isEnabled": {
              "description": "Open a pull request for the work of a task. When it is off the work is merged locally. When it is on a run is refused unless the provider CLI is installed and logged in and every repository has an origin remote on the provider.",
              "type": "boolean",
              "default": false
            },
            "isDraft": {
              "description": "Open pull requests as drafts.",
              "type": "boolean",
              "default": false
            },
            "titleFormat": {
              "description": "How the agent is told to write the title of a pull request, word for word. {{ticketID}} and {{taskTitle}} are filled in first.",
              "type": "string",
              "default": "<a short summary of the change>"
            },
            "templateFile": {
              "description": "File name of the pull request body template used for a repository that has none, resolved under ~/.drudge/prompts/. Omit to use the built-in template.",
              "type": "string"
            },
            "stepsFile": {
              "description": "File name of the pull request steps the prompt is given in place of {{pullRequestSteps}}, resolved under ~/.drudge/prompts/. It may use the {{titleFormat}}, {{templatePaths}} and {{defaultTemplate}} placeholders. Omit to use the built-in steps.",
              "type": "string"
            }
          },
          "additionalProperties": false
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
        },
        "branchFormat": {
          "description": "Format of the names of the branches DRUDGE makes for tasks. Literal text mixes with the placeholders {{taskShortID}}, the leading characters of the task id, and {{taskSlug}}, the task title folded to lowercase words joined by -. A format needs {{taskShortID}} or {{taskSlug}}. Once the placeholders are filled in, repeated / collapse into one, -, _, . and / are trimmed from both ends of every segment and empty segments are dropped. A changed format applies from the next run. Branches made before stay as they are.",
          "type": "string",
          "default": "drudge/{{taskShortID}}-{{taskSlug}}"
        },
        "pageSize": {
          "description": "How many tasks drg task list shows on one page. 0 shows every task on one page.",
          "type": "integer",
          "minimum": 0,
          "default": 20
        }
      },
      "additionalProperties": false
    },
    "drudger": {
      "description": "Settings for the Drudgers. Each one overrides the global config.",
      "type": "object",
      "properties": {
        "pageSize": {
          "description": "How many Drudgers drg drudger list shows on one page. 0 shows every Drudger on one page.",
          "type": "integer",
          "minimum": 0,
          "default": 20
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
    },
    "remote": {
      "description": "Settings for the provider the repositories of a project are hosted on. Each one overrides the global config.",
      "type": "object",
      "properties": {
        "provider": {
          "description": "The provider the repositories are hosted on. Required when pull requests are on.",
          "type": "string",
          "enum": ["github"]
        },
        "timeoutSeconds": {
          "description": "Seconds allowed for one call of the provider CLI.",
          "type": "integer",
          "minimum": 1,
          "default": 60
        },
        "pullRequests": {
          "description": "Settings for the pull requests DRUDGE opens.",
          "type": "object",
          "properties": {
            "isEnabled": {
              "description": "Open a pull request for the work of a task. When it is off the work is merged locally. When it is on a run is refused unless the provider CLI is installed and logged in and every repository has an origin remote on the provider.",
              "type": "boolean",
              "default": false
            },
            "isDraft": {
              "description": "Open pull requests as drafts.",
              "type": "boolean",
              "default": false
            },
            "titleFormat": {
              "description": "How the agent is told to write the title of a pull request, word for word. {{ticketID}} and {{taskTitle}} are filled in first.",
              "type": "string",
              "default": "<a short summary of the change>"
            },
            "templateFile": {
              "description": "File name of the pull request body template used for a repository that has none, resolved under .drudge/prompts/ of the project directory. Omit to use the global template file.",
              "type": "string"
            },
            "stepsFile": {
              "description": "File name of the pull request steps the prompt is given in place of {{pullRequestSteps}}, resolved under .drudge/prompts/ of the project directory. It may use the {{titleFormat}}, {{templatePaths}} and {{defaultTemplate}} placeholders. Omit to use the global steps file.",
              "type": "string"
            }
          },
          "additionalProperties": false
        }
      },
      "additionalProperties": false
    }
  },
  "required": ["projectSlug"],
  "additionalProperties": false
}`

// LocalSchema returns the bundled JSON schema for the local config file.
func LocalSchema() []byte {
	return []byte(localSchemaJSON)
}
