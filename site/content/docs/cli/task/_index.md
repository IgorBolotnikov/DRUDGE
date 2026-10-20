---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task
linkTitle: task
weight: 4
generated: true
---

```text
usage: drg task <subcommand>
```

Task management commands

## Subcommands

| Command | Description |
| --- | --- |
| [`drg task new`]({{< relref "/docs/cli/task/new" >}}) | Add a task to the project |
| [`drg task list`]({{< relref "/docs/cli/task/list" >}}) | List the tasks of the project |
| [`drg task next`]({{< relref "/docs/cli/task/next" >}}) | Print the oldest todo task that is ready to run |
| [`drg task show`]({{< relref "/docs/cli/task/show" >}}) | Print one task in full |
| [`drg task edit`]({{< relref "/docs/cli/task/edit" >}}) | Change the fields of a task |
| [`drg task done`]({{< relref "/docs/cli/task/done" >}}) | Mark an unmerged task done once its work is merged |
| [`drg task rm`]({{< relref "/docs/cli/task/rm" >}}) | Delete a task |
| [`drg task run`]({{< relref "/docs/cli/task/run" >}}) | Hand a task in todo status to a coding agent |
| [`drg task rerun`]({{< relref "/docs/cli/task/rerun" >}}) | Start a task over from scratch |
| [`drg task pr`]({{< relref "/docs/cli/task/pr" >}}) | Open the pull requests close-out left behind for a task |
| [`drg task status`]({{< relref "/docs/cli/task/status" >}}) | Tell how the agent working on a task is doing |
| [`drg task runlog`]({{< relref "/docs/cli/task/runlog" >}}) | Print the logs of the latest run of a task |
| [`drg task export`]({{< relref "/docs/cli/task/export" >}}) | Write every task of a project into a zip archive |
| [`drg task import`]({{< relref "/docs/cli/task/import" >}}) | Add the tasks of a zip archive the project does not have yet |
