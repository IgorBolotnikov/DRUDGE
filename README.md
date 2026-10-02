<p align="center">
  <picture>
    <img src="assets/logo.svg" alt="DRUDGE" />
  </picture>
</p>

<!--toc:start-->

- [Features](#features)
- [Install](#install)
  - [Single command](#single-command)
  - [Go](#go)
  - [Update](#update)
- [Pull requests](#pull-requests)
- [Future improvements](#future-improvements)

<!--toc:end-->

DRUDGE runs your backlog through coding agents, each in its own sandbox, on your own machine.

You make the decisions and review what comes back. The agents do the grunt work in between.

They're not coworkers. They're not digital employees. They're tools.

## Features

- Projects that link a local directory and its git repositories
- Tasks stored as plain markdown files
- Task dependencies, ensuring work starts only once its blockers are done
- Tasks grouped by ticket (external ID) and by parent task
- A pool of reusable drudgers, each with its own git worktree per repository
- Several drudgers working in parallel, you configure how many
- Claude Code as the agent (others TBD) vendor
- A full record of every run: the prompt, the event stream, the logs, the cost and the time it took to work on a task
- Tasks go back to the queue when the vendor turns an agent away, whether by rate limit, outage or expired login
- Commands to reclaim a stuck sandbox or nuke a broken one
- A bundled skill that lets a planning agent write DRUDGE tasks for you
- Color themes and a config file with a JSON schema

## Install

DRUDGE runs on Linux and macOS. On Windows, use WSL.

### Single command

```sh
curl -fsSL https://drgtools.dev/install.sh | sh
```

The script puts `drg` in `~/.local/bin`, or in `/usr/local/bin` when run as root. Set `DRG_VERSION=v0.1.0` to install a specific release, or `DRG_INSTALL_DIR` to pick another directory.

### Go

```sh
go install github.com/IgorBolotnikov/DRUDGE/cmd/drg@latest
```

Then run `drg setup`.

### Update

```sh
drg update
```

It replaces the binary with the latest release and checks it against the release checksums. A binary built from source is updated by building it again.

## Pull requests

With `remote.pullRequests.isEnabled` on, the prompt tells the agent to write a pull request description to `.drudge/pull-request.md` at the root of every repository it committed to. Before every run DRUDGE adds `.drudge/` to `.git/info/exclude` of each repository and deletes the description the last run left there.

A custom prompt file must use the `{{pullRequestSteps}}` placeholder when pull requests are on. DRUDGE refuses to start a run without it. With pull requests off the placeholder expands to nothing.

These fields go under `remote.pullRequests` in the global or the local config. The local config overrides each field it sets.

- `titleFormat` is how the agent writes the title, handed over word for word after `{{ticketID}}` and `{{taskTitle}}` are filled in. The default is `<a short summary of the change>`.
- `templateFile` is a file name in the prompts directory. It replaces the built-in body template the agent uses for a repository that has no pull request template.
- `stepsFile` is a file name in the prompts directory. It replaces the wording of the whole `{{pullRequestSteps}}` block and may use the `{{titleFormat}}`, `{{templatePaths}}` and `{{defaultTemplate}}` placeholders.

```json
{
  "remote": {
    "provider": "github",
    "pullRequests": {
      "isEnabled": true,
      "titleFormat": "{{ticketID}}: <a short summary of the change>",
      "templateFile": "pull-request-template.md"
    }
  }
}
```

## Future improvements

- TUI
- Extensibility with plugins
- More agent vendors
