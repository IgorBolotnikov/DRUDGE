# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```
go build ./...                          # build everything
go run ./cmd/drg <args>                 # run the CLI without building
make drg <args>                         # same, via Makefile (e.g. make drg task list)
go test ./...                           # run all tests
go test ./internal/task/...             # run tests for one package
go test ./internal/task/ -run TestName  # run a single test
make site                               # serve the docs site with hugo server (needs Hugo extended)
go test ./internal/cmd -run TestCLIReference -update  # regenerate the CLI reference in site/content/docs/cli/
```

## What this is

DRUDGE is a self-hosted control plane for coding agents: give it a backlog, it hands work to isolated agents, watches them, runs checks, and repeats. See `README.md`.

## Minimal maintenance

DRUDGE is built to need as little maintenance and support as possible. This applies to the tool and to its docs.

- The tool is explicit and configurable. It fails with a clear error that says what went wrong and what to do about it. Behavior a user may want to change is a config key with a documented default.
- Docs are generated from code wherever possible. Reference pages for commands and config keys are rendered from the command tree and the bundled JSON schemas by golden-file tests, and `go test ./...` fails when a committed page drifts from the code. Only what code can't express is written by hand. Those pages stay short and link to the generated reference for details.

## Architecture

This codebase follows domain-driven design. `internal/cmd` is the thin CLI layer, and the CLI is only one of several planned interfaces — a TUI and an HTMX-based web frontend are planned on top of the same domain layer. Business logic belongs in the domain packages (`internal/project`, `internal/task`, `internal/drudger`,their services) and never in `internal/cmd`. A command handler should only parse args, call a service, and format output — if a handler is doing anything more than that, it's logic that another interface will need to duplicate, and it should move down into the domain layer instead.

**Local-first and unix-only.** Sandboxes are local containers, workspaces are local directories and run artifacts live in local files. Concurrency means two CLI invocations on one machine. Use same-machine primitives like `flock` and local paths. Don't add networked storage, cross-machine coordination or distributed locking. Windows users run DRUDGE in WSL, so don't add Windows build tags or fallbacks.

**CLI dispatch.** `cmd.NewRoot` (`internal/cmd/cmd.go`) builds the tree of `*cmd.Cmd` declarations and `main.go` calls `Validate` and `Execute` on it. A command declares its `Args` (the positionals it requires), `Desc`, `Help`, `Subcommands` and `Setup`. `Setup` declares the flags of the command on a per-command `flag.FlagSet` and returns its run function. `Execute` walks the tree and handles help, positional counts and flags interleaved with positionals, so a run function gets parsed flags and exactly the positionals it declared. `Validate` panics on a leaf without `Setup`. A command with non-trivial flags keeps them in a flag struct with a `declare(fs)` method and a method that turns the flags into the DTO of its service, like `taskNewFlags` in `internal/cmd/task.go`. Only the standard library is used. Never use the global `flag.CommandLine` and don't introduce a flags/cobra-style dependency.

**Domain packages are ports-and-adapters.** `internal/project`, `internal/task` and `internal/drudger` each follow the same three-file shape:

- `model.go` — the entity struct
- `repository.go` — a `CreateXDto` plus a `XRepository` interface (the port)
- `service.go` — a `XService` holding a `XRepository` + `common.Progress`, doing validation and orchestration
- optionally, if a service is large enough, it is split among multiple files named after a facet they implement, like `internal/drudger` does

`internal/adapters/persistence` provides the filesystem-backed implementation of those repository interfaces. CLI commands wire a repo + logger into a service directly (no DI container) — see `taskNew`/`projectInit` in `internal/cmd`.

**Storage layout** (see `docs/DOMAIN_MODEL.md`), all under `~/.drudge/`:

- `~/.drudge/projects/<slug>/project.json` — one project record
- `~/.drudge/projects/<slug>/tasks/<uuid> <title>.md` — one task per file, stored as markdown with a front-matter header (`common.FormatFrontMatter`/`ParseFrontMatter` in `internal/common/fs.go`) and the task description as the raw body. Front matter is a flat `key: value` block between `---` lines, not real YAML. Every write goes through `UpdateTask`, which re-reads the file under a `<uuid>.lock` file in the same directory, so two commands cannot overwrite each other's fields.
- `~/.drudge/projects/<slug>/drudgers.json` — the Drudgers a project has, written by `FileDrudgerRepository` under a lock file beside it. An entry records the slot, the sandbox name, the workspace the agent works in, the task occupying it, what drudge last saw of its sandbox, of its workspace and of its agent, and when drudge last looked. It is the authority on which slots are free, so a task record never carries one.
- `./.drudge/config.json` (in the current working directory) — links a local directory to a project slug, written by `drg project init` and read by task commands to figure out "the project I'm in". It also records the git repositories of the project.
- `./.drudge/worktrees/slot-<n>/<repo-path>` — the workspace of one Drudger, a git worktree per repository of the project. It is made with the Drudger, mounted into its sandbox, and outlives every Session that runs in it.
- `./.drudge/runs/<task-id>` — the prompt, the event stream, the stderr log and the exit code of one run, and the pull request descriptions close-out has not opened a pull request from yet.

**Config and theme** are separate concerns, both under `~/.drudge/`:

- `internal/config` — `GlobalConfig` (Drudger environment/harness), defaults merged with whatever's on disk, bundled JSON schema in `schema.go`
- `internal/theme` — terminal color theme. Bundled palettes (`nord`, `monokai`, `catppuccin-mocha`, `dracula`, and `system` made of basic ANSI colors and dim) in `themes.go`, keyed by role (`primary`, `error`, `success`, ...), overridable per-role via `~/.drudge/theme.json`, rendered as 24-bit ANSI escapes. Invalid override colors are logged and skipped, not treated as fatal. The theme alone decides whether color is on, so no other code checks `NO_COLOR` or the terminal for color.

Both write their bundled JSON schema files to `~/.drudge/schema/` via `drg setup` (`internal/setup`), and both config files carry a `$schema` pointer to that local file.

**`internal/common`** holds the shared low-level helpers everything else depends on: path helpers for the `~/.drudge` layout, generic JSON read/write, the front-matter format, UUID generation, and a minimal `Logger` (`Info` → stdout, `Warn` and `Error` → stderr, optional bracketed prefix). Don't reintroduce these primitives in a domain package.

**Docs site.** `site/` is the Hugo project published at `https://drgtools.dev/` by `.github/workflows/docs.yaml`. It has its own `go.mod`, so `go build ./...` and `go test ./...` skip it. The Hextra theme comes in as a Hugo module pinned in `site/go.mod`. `site/hugo.yaml` mounts the root `install.sh` and `assets/logo.svg`, so those files have one copy each. User docs live in `site/content/docs/`. The key tables of `site/content/docs/configuration/config.md`, `project-config.md` and `theme.md` are generated from the bundled JSON schemas and sit between the `BEGIN GENERATED` and `END GENERATED` markers. Everything outside the markers is hand-written. Regenerate the tables with `go test ./internal/config ./internal/theme -update`. `docs/` holds notes for contributors and agents and is never published.

`site/content/docs/cli/` is the CLI reference, generated from the `drg` command tree by `TestCLIReference` in `internal/cmd`. Never edit it by hand. Regenerate it with `go test ./internal/cmd -run TestCLIReference -update` after changing a command. `site/layouts/_partials/toc.html` is a copy of the Hextra partial that leaves the edit link off generated pages, so update it along with the Hextra version.

A change to user-facing behavior updates the matching page under `site/content/docs/` in the same commit.

## Vocabulary

Call things what they physically are, and keep status language blunt. We don't do bullshit corporate speak here.

Task statuses read like verdicts, not events: prefer "got shit done" / "fucked up" / "needs babysitting" over "completed successfully" / "encountered an unexpected error" / "requires human intervention"

This vocabulary is reflected in the domain model itself — e.g. `task.StatusFuckedUp` in `internal/task/model.go`.

The domain nouns are defined in `docs/TERMINOLOGY.md`: **Drudger** (an agent running in a reusable sandbox that does the work), **Session** (a single run of work on a task by one Drudger), **Task** (a piece of work to be done). Read it before naming anything.

If we were to put this project on a scale of AI psychosis from 0 to 5, it would be a fucking -1.

## Code and comment style

The rules for writing code, comments, commit messages and branch names live in the `code-style` skill. Invoke it before you edit any file in this repo, and again before you write a commit message.

## Roadmap docs

`docs/TASK_IMPLEMENTATION.md` and `TODO.md` describe planned behavior that isn't implemented yet (e.g. `drg task show`, automatic status transitions with timestamps, git branch setup per task). Check whether a feature actually exists in `internal/cmd`/`internal/task` before assuming these docs describe current behavior.
