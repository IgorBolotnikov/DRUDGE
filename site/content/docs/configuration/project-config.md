---
title: Project config
weight: 2
---

The project config file is `.drudge/config.json` in the project directory. `drg project init <name>` writes it with the project slug, the git repositories it finds with the setup it detects for each, and the `$schema` pointer. Every task, drudger and run command reads it to find the project of the current directory, so they fail in a directory without one.

## Schema

The file starts with a `$schema` key holding the absolute path of `~/.drudge/schema/local-config.json`. The file lives outside `~/.drudge/`, so a relative path would not resolve. `drg setup` rewrites the schema file on every run. An editor with JSON schema support uses it to complete keys and flag wrong values.

## Defaults

A key set here overrides the same key of the [global config](../config/). `promptFile` and `maxConcurrentDrudgers` override `drudger.promptFile` and `drudger.maxConcurrentDrudgers`. A key left out falls back to the global config, then to the default in the table below. A value DRUDGE does not accept stops the command with an error that names the file and the key.

In a terminal `drg project init` asks for `maxConcurrentDrudgers`, `task.branchFormat` and `remote.pullRequests.isEnabled`, and for `remote.pullRequests.isDraft` when pull requests are on. The branch format is a pick of `drudge/{{taskShortID}}-{{taskSlug}}`, `{{ticketID}}-{{taskSlug}}`, `feat/{{ticketID}}/{{taskSlug}}` and Custom…, which asks for the format as text. Every question starts on the value the global config resolves to. Init writes only the answers that differ from it, so a later change to the global config still reaches every key the project leaves out. Pull requests turned on with no `remote.provider` in the global config write `remote.provider: "github"` too.

A `promptFile`, `remote.pullRequests.templateFile` or `remote.pullRequests.stepsFile` set here is a file name under `.drudge/prompts/` of the project directory.

## Setup

Each entry of `repositories` can list `setup` commands, like `npm ci` or `make deps`. A launch runs them in the sandbox of the Drudger after it puts the worktrees on the task branch and before it starts the agent. They run for every repository the task changes, in config order, each with `sh -c` in the worktree of its repository. The output goes to `setup.log` in the run directory. The first command that fails stops the launch and the task stays where it was. `sandboxTimeouts.setupSeconds` of the [global config](../config/) caps each command. `drg task run --dry-run` lists the commands without running them.

`drg project detect` proposes the setup of each repository from the files in its root and writes it into the config. A version file like `.tool-versions`, `mise.toml`, `.nvmrc` or `.python-version` gets mise and the pinned toolchains installed. The first lockfile found, like `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `uv.lock`, `poetry.lock` or `requirements.txt`, gets its packages installed. A `.pre-commit-config.yaml` gets pre-commit installed with uv, unless a version file pins it, and its hooks installed through `DRUDGE_HOOKS`. A worktree shares the hooks directory of its host repository, so the hooks path is set for the worktree only and the host repository keeps its own hooks. Hooks a package manager installs, like husky, get no lines. It fills only the repositories with no `setup` key, `--force` replaces every one and `--dry-run` writes nothing. `drg project init` runs the same detection. An empty list means there is nothing to do. A missing key means the setup is not decided yet. It runs nothing too, and a launch and `drg task run --dry-run` warn about it. See [`drg project detect`]({{< relref "/docs/cli/project/detect" >}}) for the details.

Every setup command gets two environment variables:

- `DRUDGE_PATH` names the file `.drudge/path` under the sandbox home. A launch empties it before the first setup command. A command adds a directory to PATH by appending one line to the file. The later setup commands of the launch and the agent get every listed directory prepended to their PATH, in file order.
- `DRUDGE_HOOKS` names a git hooks directory of the repository under `.drudge/hooks/` of the sandbox home, outside the worktree. A launch creates it before the setup of the repository runs. Every repository gets a directory of its own.

```json
"setup": [
  "curl -fsSL https://mise.run | sh",
  "echo \"$HOME/.local/bin\" >> \"$DRUDGE_PATH\"",
  "mise install"
]
```

## Copied files

A worktree is a fresh checkout, so gitignored files like `.env` never reach it. Each entry of `repositories` can list `copy` paths relative to the repository. On every launch `drg` copies each one from the repository into the worktree before setup runs, for every repository the task changes. A file overwrites the one in the worktree and a directory is copied whole. A path the repository does not have is a warning and the launch goes on. `drg task run --dry-run` lists the paths without copying them.

`drg project detect` and `drg project init` fill `copy` together with `setup`. They propose the files in the root of the repository that git ignores and whose name starts with `.env`, like `.env` and `.env.local`, and a `.envs` directory. A tracked file like `.env.example` is left out, and `.envrc` is never proposed. A repository with a `setup` key keeps its `copy` too. A repository with no `setup` key keeps a `copy` it already has. `--force` replaces both and `--dry-run` writes nothing.

## Environment

`drudger.env` lists the names of environment variables to pass from the shell of `drg` into the sandbox, like `NODE_AUTH_TOKEN` for a private package registry. Every setup command and the agent get them through `sbx exec -e NAME`, so DRUDGE never stores their values. A launch checks every name before setup runs and stops when one of them is not set. `drg task run --dry-run` lists the names and warns about each one that is not set.

`drg project detect` and `drg project init` add every `${NAME}` in the `.npmrc` at the root of a repository to `drudger.env`, like `NODE_AUTH_TOKEN` in `//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}`. They read every repository, one that keeps its `setup` included. A name already in `drudger.env` stays and no name is ever removed, `--force` included. `--dry-run` writes nothing.

## Keys

<!-- BEGIN GENERATED: edits between these markers are overwritten -->

| Key | Type | Default | Allowed | Description |
| --- | --- | --- | --- | --- |
| `$schema` | string |  |  | JSON schema reference for configuration validation |
| `projectSlug` | string |  |  | Required. Slug of the project this directory is linked to. |
| `promptFile` | string |  |  | File name of the prompt handed to an agent, resolved under .drudge/prompts/ of the project directory. Omit to use the global prompt file. |
| `maxConcurrentDrudgers` | integer | `3` | minimum 1 | How many Drudgers may work on one project at once. |
| `task` | object |  |  | Settings for tasks. Each one overrides the global config. |
| `task.defaultStatus` | string | `"todo"` | `"todo"`, `"draft"` | Status of a new task created without --status. A draft task waits for review, a todo task is ready to run. |
| `task.branchFormat` | string | `"drudge/{{taskShortID}}-{{taskSlug}}"` |  | Format of the names of the branches DRUDGE makes for tasks. Literal text mixes with the placeholders {{taskShortID}}, the leading characters of the task id, {{taskSlug}}, the task title folded to lowercase words joined by -, and {{ticketID}}, the ticket id of the task with its case kept and every character git refuses in a ref, / included, replaced by -. A format needs {{taskShortID}} or {{taskSlug}}. Once the placeholders are filled in, repeated / collapse into one, -, _, . and / are trimmed from both ends of every segment and empty segments are dropped. A changed format applies from the next run. Branches made before stay as they are. |
| `task.pageSize` | integer | `20` | minimum 0 | How many tasks drg task list shows on one page. 0 shows every task on one page. |
| `drudger` | object |  |  | Settings for the Drudgers. Each one overrides the global config. |
| `drudger.pageSize` | integer | `20` | minimum 0 | How many Drudgers drg drudger list shows on one page. 0 shows every Drudger on one page. |
| `drudger.env` | array |  |  | Names of environment variables passed from the shell of drg into the sandbox, to every setup command and to the agent. A launch stops when one of them is not set. DRUDGE never stores their values. drg project detect adds the names a .npmrc reads. |
| `repositories` | array |  |  | The git repositories of the project. drg project init writes the list. |
| `repositories[].path` | string |  |  | Required. Where the repository sits relative to the project directory. A project directory that is itself a repository records ".". |
| `repositories[].defaultBranch` | string |  |  | The branch work is cut from. Omit to read it from origin/HEAD. |
| `repositories[].setup` | array |  |  | Shell commands a launch runs in the sandbox, in the worktree of the repository, before the agent starts. Each one runs with sh -c and the first one that fails stops the launch. They run for every repository the task changes, in config order. An empty list means nothing to do. A missing key means the setup is not decided yet, it runs nothing and a launch warns about it. drg project detect fills it in. |
| `repositories[].copy` | array |  |  | Paths relative to the repository that a launch copies from the repository into its worktree before setup runs, like gitignored .env files. Files are overwritten and directories are copied whole. A path missing from the repository is a warning. drg project detect fills it in. |
| `remote` | object |  |  | Settings for the provider the repositories of a project are hosted on. Each one overrides the global config. |
| `remote.provider` | string |  | `"github"` | The provider the repositories are hosted on. Required when pull requests are on. |
| `remote.timeoutSeconds` | integer | `60` | minimum 1 | Seconds allowed for one call of the provider CLI. |
| `remote.pullRequests` | object |  |  | Settings for the pull requests DRUDGE opens. |
| `remote.pullRequests.isEnabled` | boolean | `false` |  | Open a pull request for the work of a task. When it is off the work is merged locally. When it is on a run is refused unless the provider CLI is installed and logged in and every repository has an origin remote on the provider. |
| `remote.pullRequests.isDraft` | boolean | `false` |  | Open pull requests as drafts. |
| `remote.pullRequests.titleFormat` | string | `"<a short summary of the change>"` |  | How the agent is told to write the title of a pull request, word for word. {{ticketID}} and {{taskTitle}} are filled in first. |
| `remote.pullRequests.templateFile` | string |  |  | File name of the pull request body template used for a repository that has none, resolved under .drudge/prompts/ of the project directory. Omit to use the global template file. |
| `remote.pullRequests.stepsFile` | string |  |  | File name of the pull request steps the prompt is given in place of {{pullRequestSteps}}, resolved under .drudge/prompts/ of the project directory. It may use the {{titleFormat}}, {{templatePaths}} and {{defaultTemplate}} placeholders. Omit to use the global steps file. |

<!-- END GENERATED -->
