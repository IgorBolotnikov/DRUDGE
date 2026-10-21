---
title: Getting Started
weight: 1
---

This page takes you from nothing to a first [Task]({{< relref "/docs/concepts/tasks" >}}) handed to a [Drudger]({{< relref "/docs/concepts/drudgers" >}}).

## Requirements

- Linux or macOS. On Windows, run DRUDGE inside WSL.
- git.
- [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) with the `sbx` CLI. Every Drudger runs its agent in an `sbx` sandbox.
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) with a login. sbx keeps its own copy of the token, and a host login does not refresh it. When the token inside sbx expires, log in again and re-seed the credentials inside sbx.

## Install

```sh
curl -fsSL https://drgtools.dev/install.sh | sh
```

The script puts `drg` in `~/.local/bin`, or in `/usr/local/bin` when run as root. It checks the download against the checksums of the release.

Set `DRG_VERSION` to install a specific release and `DRG_INSTALL_DIR` to pick another directory:

```sh
curl -fsSL https://drgtools.dev/install.sh | DRG_VERSION=v0.1.0 DRG_INSTALL_DIR="$HOME/bin" sh
```

With Go installed, you can build it from source:

```sh
go install github.com/IgorBolotnikov/DRUDGE/cmd/drg@latest
```

## Set up

```sh
drg setup
```

In a terminal, [`drg setup`]({{< relref "/docs/cli/setup" >}}) first asks which harness the Drudgers run. Move with ↑ and ↓ or `k` and `j`, pick with Enter, and cancel with Esc or Ctrl-C:

```
Harness
› claude-code (default)
  opencode
```

The list turns into one line with your answer, like `Harness: claude-code`.

Then it asks for the model the harness runs and for how many Drudgers may work on one project at once. Type an answer and press Enter, or press Enter alone to take the default in brackets. An answer DRUDGE refuses prints the reason and asks again:

```
Model the harness runs [harness default]:
Drudgers that may work on one project at once [3]:
```

Then come three selects. The status of a new task is `todo` or `draft`. Pull requests are off by default, and turning them on asks whether to open them as drafts:

```
Status of a new task
› todo (default)
  draft
Open pull requests
› no (default)
  yes
```

The last question is the color theme. The rest of the run keeps the colors setup started with, and the picked theme applies from the next command:

```
Color theme
› system (default)
  nord
  monokai
  catppuccin-mocha
  dracula
```

`drg setup -y`, short for `--defaults`, skips the questions and takes the defaults. A run whose input is not a terminal asks nothing either. See [Global config]({{< relref "/docs/configuration/config" >}}) and [Theme]({{< relref "/docs/configuration/theme" >}}) for the keys the answers set.

It then creates `~/.drudge/` and writes:

- `~/.drudge/config.json`, the global config
- `~/.drudge/theme.json`, the color theme
- `~/.drudge/schema/`, the JSON schemas both config files and the project config point at
- `~/.drudge/projects/`, where the projects and their Tasks live

With the `claude-code` harness it also installs the DRUDGE skill for Claude Code at `~/.claude/skills/DRUDGE/SKILL.md`. The skill lets a planning agent write DRUDGE Tasks for you.

The schemas and the skill are rewritten on every run. Running `drg setup` again in a terminal asks the same questions, starting on the values of your config files, and writes the answers back into them. Every key it does not ask about stays as it is. `drg setup -y` leaves existing config files alone.

## First project

Run [`drg project init`]({{< relref "/docs/cli/project/init" >}}) in the directory you want the agents to work on, to make it a [project]({{< relref "/docs/concepts/projects" >}}):

```sh
cd ~/code/uploader
drg project init uploader
```

A directory that is a git repository becomes the one repository of the project. Otherwise every subdirectory that is a git repository becomes one, and a directory holding none is refused.

In a terminal it then asks how many Drudgers may work on this project at once and whether to open pull requests for it. Every question starts on the value of your global config, so Enter keeps the global value:

```
Drudgers that may work on this project at once [3]:
Open pull requests
› no (default)
  yes
```

Turning pull requests on asks whether to open them as drafts. Only the answers that differ from the global config go into the project config. `drg project init uploader -y`, short for `--defaults`, skips the questions and keeps the global values. See [Project config]({{< relref "/docs/configuration/project-config" >}}) for the keys.

`drg project init` writes `.drudge/config.json` in that directory. Task commands run there work on that project. It prints each repository with the default branch that task branches are cut from. The default branch is read from `origin/HEAD`. When it shows as `unresolved`, run `git remote set-head origin -a` in the repository, or set `defaultBranch` for it in `.drudge/config.json`.

## First task

Add a Task with [`drg task new`]({{< relref "/docs/cli/task/new" >}}). Its description is the prompt the agent gets:

```sh
drg task new --title "Add retry to uploader" --description "Retry a failed upload three times before giving up."
```

It prints the short id of the Task. A Task created without `--status` is `todo`, which is ready to run. Pass `--status draft` to keep it back for review. `--description-file` reads a longer description from a file, or from stdin with `-`.

Hand it to a Drudger:

```sh
drg task run 3f9a1c2e
```

[`drg task run`]({{< relref "/docs/cli/task/run" >}}) takes the lowest free slot of the project. A slot with no Drudger yet gets one: a git worktree per repository under `.drudge/worktrees/slot-<n>/` and an sbx sandbox over them. An idle Drudger is reused. The Drudger checks out a [branch]({{< relref "/docs/concepts/close-out" >}}) for the Task, starts the agent in its sandbox and returns. `drudger.maxConcurrentDrudgers` in the global config caps how many Drudgers work on one project at once, and defaults to 3. `drg task run 3f9a1c2e --dry-run` prints the prompt the agent would get and starts nothing.

Watch it with [`drg task list`]({{< relref "/docs/cli/task/list" >}}), [`drg task show`]({{< relref "/docs/cli/task/show" >}}), [`drg task runlog`]({{< relref "/docs/cli/task/runlog" >}}) and [`drg task status`]({{< relref "/docs/cli/task/status" >}}):

```sh
drg task list                # every Task of the project and its status
drg task show 3f9a1c2e       # one Task in full, with what its last run left behind
drg task runlog 3f9a1c2e     # a compact view of the latest run, or --raw for the files the agent wrote
drg task status 3f9a1c2e     # whether the agent is working, stuck or finished
```

Once the [Session]({{< relref "/docs/concepts/sessions" >}}) has ended, `drg task status` records the outcome on the Task. A Task that got shit done ends up `done`, or `unmerged` while its work is not on the default branch yet. A Task that fucked up ends up `fucked-up`. A Task the vendor turned away goes back to `todo`. Mark an `unmerged` Task done with [`drg task done`]({{< relref "/docs/cli/task/done" >}}) once its work is merged.

[`drg drudger list`]({{< relref "/docs/cli/drudger/list" >}}) shows what each Drudger of the project is doing.

## Update and uninstall

```sh
drg update
```

[`drg update`]({{< relref "/docs/cli/update" >}}) replaces the binary with the latest release and checks it against the release checksums. A binary built from source is updated by building it again.

```sh
drg cleanup
```

[`drg cleanup`]({{< relref "/docs/cli/cleanup" >}}) asks before it removes `~/.drudge/` with every project and Task in it. `--force` skips the question. It leaves the `drg` binary, the skill under `~/.claude/skills/`, the `.drudge/` directories of your projects and the sbx sandboxes in place. Remove the sandboxes first with [`drg drudger nuke <slot>`]({{< relref "/docs/cli/drudger/nuke" >}}) in each project, then delete the binary.
