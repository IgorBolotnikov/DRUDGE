---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: CLI reference
weight: 3
generated: true
---

```text
usage: drg <subcommand>
```

Runs your backlog through coding agents, each in its own sandbox, on your own machine

## Options

| Option | Description | Default |
| --- | --- | --- |
| `--version` | Print the version of drg | `false` |

## Subcommands

| Command | Description |
| --- | --- |
| [`drg setup`]({{< relref "/docs/cli/setup" >}}) | Setup DRUDGE in this computer |
| [`drg cleanup`]({{< relref "/docs/cli/cleanup" >}}) | Cleanup DRUDGE from this computer |
| [`drg project`]({{< relref "/docs/cli/project" >}}) | Project management commands |
| [`drg task`]({{< relref "/docs/cli/task" >}}) | Task management commands |
| [`drg drudger`]({{< relref "/docs/cli/drudger" >}}) | Drudger management commands |
| [`drg update`]({{< relref "/docs/cli/update" >}}) | Update drg to the latest release |
