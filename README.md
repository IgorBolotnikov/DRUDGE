<p align="center">
  <picture>
    <img src="assets/logo.svg" alt="DRUDGE" />
  </picture>
</p>

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
- Claude Code as the agent vendor (others TBD)
- A full record of every run: the prompt, the event stream, the logs, the cost and the time it took to work on a task
- Tasks go back to the queue when the vendor turns an agent away, whether by rate limit, outage or expired login
- Commands to reclaim a stuck sandbox or nuke a broken one
- A bundled skill that lets a planning agent write DRUDGE tasks for you
- Color themes and a config file with a JSON schema

## Install

```sh
curl -fsSL https://drgtools.dev/install.sh | sh
```

Other install options, setup and your first task are in the docs at https://drgtools.dev/docs/.
