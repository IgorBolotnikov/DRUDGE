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
```

## What this is

DRUDGE is a self-hosted control plane for coding agents: give it a backlog, it hands work to isolated agents, watches them, runs checks, and repeats. See `README.md`.

## Architecture

This codebase follows domain-driven design. `internal/cmd` is the thin CLI layer, and the CLI is only one of several planned interfaces — a TUI and an HTMX-based web frontend are planned on top of the same domain layer. Business logic belongs in the domain packages (`internal/project`, `internal/task`, `internal/drudger`,their services) and never in `internal/cmd`. A command handler should only parse args, call a service, and format output — if a handler is doing anything more than that, it's logic that another interface will need to duplicate, and it should move down into the domain layer instead.

**Local-first and unix-only.** Sandboxes are local containers, workspaces are local directories and run artifacts live in local files. Concurrency means two CLI invocations on one machine. Use same-machine primitives like `flock` and local paths. Don't add networked storage, cross-machine coordination or distributed locking. Windows users run DRUDGE in WSL, so don't add Windows build tags or fallbacks.

**CLI dispatch.** `cmd.NewRoot` (`internal/cmd/cmd.go`) builds a tree of `*cmd.Cmd` declarations and `main.go` calls `Validate` and `Execute` on it. There's no third-party CLI framework: `Execute` walks the tree, parses the flags a command declares in `Setup` with a per-command stdlib `flag.FlagSet`, checks the positionals against `Args` and prints generated help. Never use the global `flag.CommandLine`. A legacy leaf (`Run` set, no `Setup`) gets the raw args and does its own `switch args[0]` dispatch and hand-rolled flag parsing (see `parseFlagValue`/`hasFlag` in `internal/cmd/task.go`); those commands move onto `Setup` one group at a time. Don't introduce a flags/cobra-style dependency.

**Domain packages are ports-and-adapters.** `internal/project`, `internal/task` and `internal/drudger` each follow the same three-file shape:

- `model.go` — the entity struct
- `repository.go` — a `CreateXDto` plus a `XRepository` interface (the port)
- `service.go` — a `XService` holding a `XRepository` + `*common.Logger`, doing validation and orchestration
- optionally, if a service is large enough, it is split among multiple files named after a facet they implement, like `internal/drudger` does

`internal/adapters/persistence` provides the filesystem-backed implementation of those repository interfaces. CLI commands wire a repo + logger into a service directly (no DI container) — see `taskNew`/`projectCreate` in `internal/cmd`.

**Storage layout** (see `docs/DOMAIN_MODEL.md`), all under `~/.drudge/`:

- `~/.drudge/projects/<slug>/project.json` — one project record
- `~/.drudge/projects/<slug>/tasks/<uuid> <title>.md` — one task per file, stored as markdown with a front-matter header (`common.FormatFrontMatter`/`ParseFrontMatter` in `internal/common/fs.go`) and the task description as the raw body. Front matter is a flat `key: value` block between `---` lines, not real YAML. Every write goes through `UpdateTask`, which re-reads the file under a `<uuid>.lock` file in the same directory, so two commands cannot overwrite each other's fields.
- `~/.drudge/projects/<slug>/drudgers.json` — the Drudgers a project has, written by `FileDrudgerRepository` under a lock file beside it. An entry records the slot, the sandbox name, the workspace the agent works in, the task occupying it, what drudge last saw of its sandbox, of its workspace and of its agent, and when drudge last looked. It is the authority on which slots are free, so a task record never carries one.
- `./.drudge/config.json` (in the current working directory) — links a local directory to a project slug, written by `drg project init` and read by task commands to figure out "the project I'm in". It also records the git repositories of the project.
- `./.drudge/worktrees/slot-<n>/<repo-path>` — the workspace of one Drudger, a git worktree per repository of the project. It is made with the Drudger, mounted into its sandbox, and outlives every Session that runs in it.
- `./.drudge/runs/<task-id>` — the prompt, the event stream, the stderr log and the exit code of one run.

**Config and theme** are separate concerns, both under `~/.drudge/`:

- `internal/config` — `GlobalConfig` (Drudger environment/harness), defaults merged with whatever's on disk, bundled JSON schema in `schema.go`
- `internal/theme` — terminal color theme. Bundled palettes (`nord`, `monokai`, `catppuccin-mocha`, `dracula`) in `themes.go`, keyed by role (`primary`, `error`, `success`, ...), overridable per-role via `~/.drudge/theme.json`, rendered as 24-bit ANSI escapes. Invalid override colors are logged and skipped, not treated as fatal.

Both write their bundled JSON schema files to `~/.drudge/schema/` via `drg setup` (`internal/cmd/setup.go`), and both config files carry a `$schema` pointer to that local file.

**`internal/common`** holds the shared low-level helpers everything else depends on: path helpers for the `~/.drudge` layout, generic JSON read/write, the front-matter format, UUID generation, and a minimal `Logger` (`Info` → stdout, `Error` → stderr, optional bracketed prefix). Don't reintroduce these primitives in a domain package.

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
