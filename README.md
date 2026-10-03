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

With `remote.pullRequests.isEnabled` on, DRUDGE opens a pull request for every repository a run committed to. It works like this:

1. Before a run DRUDGE checks that `gh` is installed and logged in, and that every repository has an `origin` remote on GitHub. It refuses to start the run otherwise.
2. The prompt tells the agent to write a pull request description to `.drudge/pull-request.md` at the root of every repository it committed to. Before every run DRUDGE adds `.drudge/` to `.git/info/exclude` of each repository and deletes the description the last run left there.
3. When a run that got shit done is recorded, DRUDGE moves each description into the run directory as `.drudge/runs/<task-id>/pull-requests/<repository>.md`.
4. For each description DRUDGE pushes the branch of the run to `origin` and opens a pull request from it into the default branch of the repository with `gh pr create`. The first line of the description is the title, with a leading `# ` cut. The rest is the body. A description with a blank first line takes the title of the task.
5. The URL of every opened pull request goes on the task and its description is deleted from the run directory. `drg task show` lists the URLs.

A repository with commits and no description is not pushed and gets no pull request. DRUDGE prints a warning for it. A push or a pull request that fails is printed with the reason, and its description stays in the run directory. The other repositories still get their pull requests. A run that fucked up opens nothing.

`drg task pr <task-id>` opens the pull requests whose descriptions are still in the run directory. It retries the ones that failed and opens them for a task that finished before pull requests were turned on. It runs the same check as `drg task run` and refuses a task an agent is still working on. Each branch is pushed from the repository in the project directory, so it works whatever task the Drudger holds now. A repository the task has work in and no description left for is printed. Its pull request is open already or the agent never wrote a description.

The task ends as `unmerged` either way. DRUDGE never reads the state of a pull request. Run `drg task done` once the work is merged. A rerun of the task deletes the run directory with the descriptions left in it, and leaves the pull requests of earlier runs open.

A custom prompt file must use the `{{pullRequestSteps}}` placeholder when pull requests are on. DRUDGE refuses to start a run without it. With pull requests off the placeholder expands to nothing.

These fields go under `remote.pullRequests` in the global or the local config. The local config overrides each field it sets.

- `isDraft` opens every pull request as a draft.
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
