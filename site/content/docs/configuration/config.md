---
title: Global config
weight: 1
---

The global config file is `~/.drudge/config.json`. `drg setup` writes it when it is missing, with the Drudger environment and harness set. After that the file is yours to edit. `drg setup` leaves an existing file alone.

In a terminal, `drg setup` asks which harness the Drudgers run before it writes the file. `--defaults`, or `-y`, skips the questions and writes the defaults. Input that is not a terminal skips them too.

## Schema

The file starts with `"$schema": "./schema/config.json"`, a path relative to the file. It points at `~/.drudge/schema/config.json`, the local copy of the schema that `drg setup` rewrites on every run. An editor with JSON schema support uses it to complete keys and flag wrong values.

## Defaults

Every command reads the file again, so a change applies from the next command. A missing file means every key takes its default. A key left out of the file takes the default in the table below. A key without a default stays unset. A value DRUDGE does not accept stops the command with an error that names the file and the key.

The `task` and `remote` keys, `drudger.pageSize`, `drudger.promptFile` and `drudger.maxConcurrentDrudgers` can be overridden per project in the [project config](../project-config/).

## Keys

<!-- BEGIN GENERATED: edits between these markers are overwritten -->

| Key | Type | Default | Allowed | Description |
| --- | --- | --- | --- | --- |
| `$schema` | string |  |  | JSON schema reference for configuration validation |
| `drudger` | object |  |  | Settings for the Drudgers that execute tasks. |
| `drudger.environment` | string | `"docker-sbx"` | `"docker-sbx"` | The environment tasks run in. |
| `drudger.harness` | string | `"claude-code"` | `"claude-code"`, `"opencode"` | The agent harness used to run tasks. |
| `drudger.promptFile` | string |  |  | File name of the prompt handed to an agent, resolved under ~/.drudge/prompts/. Omit to use the built-in default prompt. |
| `drudger.model` | string |  |  | The model the harness runs, passed to it as is. For Claude Code it is an alias like opus or sonnet, or a full model name like claude-opus-5-5. Omit to leave the choice to the harness. |
| `drudger.maxConcurrentDrudgers` | integer | `3` | minimum 1 | How many Drudgers may work on one project at once. |
| `drudger.pageSize` | integer | `20` | minimum 0 | How many Drudgers drg drudger list shows on one page. 0 shows every Drudger on one page. |
| `drudger.sandboxTimeouts` | object |  |  | How long DRUDGE waits for a sandbox command before it kills it. A command that needs longer than its timeout is killed and reported. |
| `drudger.sandboxTimeouts.listSeconds` | integer | `30` | minimum 1 | Seconds allowed for listing the sandboxes. |
| `drudger.sandboxTimeouts.createSeconds` | integer | `600` | minimum 1 | Seconds allowed for creating a sandbox. A first creation pulls an image, which is why this one is generous. |
| `drudger.sandboxTimeouts.removeSeconds` | integer | `120` | minimum 1 | Seconds allowed for removing a sandbox. |
| `drudger.sandboxTimeouts.setupSeconds` | integer | `900` | minimum 1 | Seconds allowed for each setup command of a repository, run in the sandbox before the agent starts. |
| `drudger.gitTimeouts` | object |  |  | How long DRUDGE waits for a git command before it kills it. Git has no timeout of its own. |
| `drudger.gitTimeouts.fetchSeconds` | integer | `15` | minimum 1 | Seconds allowed for fetching from a remote. Kept short so an unreachable remote costs seconds. |
| `drudger.gitTimeouts.worktreeSeconds` | integer | `600` | minimum 1 | Seconds allowed for creating a worktree, which is a full checkout. |
| `drudger.gitTimeouts.commandSeconds` | integer | `60` | minimum 1 | Seconds allowed for every other git command. |
| `task` | object |  |  | Settings for tasks. A project config may override each of them. |
| `task.defaultStatus` | string | `"todo"` | `"draft"`, `"todo"` | Status of a new task created without --status. A draft task waits for review, a todo task is ready to run. |
| `task.branchFormat` | string | `"drudge/{{taskShortID}}-{{taskSlug}}"` |  | Format of the names of the branches DRUDGE makes for tasks. Literal text mixes with the placeholders {{taskShortID}}, the leading characters of the task id, {{taskSlug}}, the task title folded to lowercase words joined by -, and {{ticketID}}, the ticket id of the task with its case kept and every character git refuses in a ref, / included, replaced by -. A format needs {{taskShortID}} or {{taskSlug}}. Once the placeholders are filled in, repeated / collapse into one, -, _, . and / are trimmed from both ends of every segment and empty segments are dropped. A changed format applies from the next run. Branches made before stay as they are. |
| `task.pageSize` | integer | `20` | minimum 0 | How many tasks drg task list shows on one page. 0 shows every task on one page. |
| `project` | object |  |  | Settings for projects. |
| `project.pageSize` | integer | `20` | minimum 0 | How many projects drg project list shows on one page. 0 shows every project on one page. |
| `remote` | object |  |  | Settings for the provider the repositories of a project are hosted on. A project config may override each of them. |
| `remote.provider` | string |  | `"github"` | The provider the repositories are hosted on. Required when pull requests are on. |
| `remote.timeoutSeconds` | integer | `60` | minimum 1 | Seconds allowed for one call of the provider CLI. |
| `remote.pullRequests` | object |  |  | Settings for the pull requests DRUDGE opens. |
| `remote.pullRequests.isEnabled` | boolean | `false` |  | Open a pull request for the work of a task. When it is off the work is merged locally. When it is on a run is refused unless the provider CLI is installed and logged in and every repository has an origin remote on the provider. |
| `remote.pullRequests.isDraft` | boolean | `false` |  | Open pull requests as drafts. |
| `remote.pullRequests.titleFormat` | string | `"<a short summary of the change>"` |  | How the agent is told to write the title of a pull request, word for word. {{ticketID}} and {{taskTitle}} are filled in first. |
| `remote.pullRequests.templateFile` | string |  |  | File name of the pull request body template used for a repository that has none, resolved under ~/.drudge/prompts/. Omit to use the built-in template. |
| `remote.pullRequests.stepsFile` | string |  |  | File name of the pull request steps the prompt is given in place of {{pullRequestSteps}}, resolved under ~/.drudge/prompts/. It may use the {{titleFormat}}, {{templatePaths}} and {{defaultTemplate}} placeholders. Omit to use the built-in steps. |

<!-- END GENERATED -->
