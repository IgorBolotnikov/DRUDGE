---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg project init
linkTitle: init
weight: 1
generated: true
---

```text
usage: drg project init <name> [options]
```

Record a project and link the current directory to it. Task commands run here work on that project.

A directory that is a git repository becomes the one repository of the project.

Otherwise every subdirectory that is a git repository becomes one, and a directory holding none is refused.

It writes the setup commands, copy paths and environment variable names it detects for each repository, like drg project detect does.

In a terminal it asks how many Drudgers may work on the project at once and whether to open pull requests, each question starting on the value of the global config.

It writes into the local config only the answers that differ from the global config. Pull requests turned on with no remote.provider in the global config write remote.provider too.

With --defaults or input that is not a terminal it asks nothing and every setting falls back to the global config.

It prints each repository with the default branch that task branches are cut from.

## Arguments

- `<name>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-y, --defaults` | Skip the questions and take the values of the global config | `false` |
