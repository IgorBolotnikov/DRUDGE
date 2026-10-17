---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task edit
linkTitle: edit
weight: 5
generated: true
---

```text
usage: drg task edit <task-id> [options]
```

Change what a task asks for and where it stands. It takes at least one field.

The task ID may be the short one a listing prints, as long as it names a single task.

## Arguments

- `<task-id>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `--add-pull-request <urls>` | Comma-separated urls of pull requests to add to the ones opened for this task |  |
| `--add-repo <paths>` | Comma-separated paths of repositories to add to the ones this task changes |  |
| `--block <ids>` | Comma-separated ids of tasks to add to the ones this task waits for |  |
| `--blocked-by <ids>` | Comma-separated ids of the tasks this task waits for, replacing the list, empty to clear it |  |
| `--description <text>` | New description, the text the agent is handed as its prompt |  |
| `--description-file <path>` | Read the new description from the file at path, - to read stdin |  |
| `-f, --force` | Set a status drudge maintains itself (in-progress, fucked-up, unmerged) | `false` |
| `--parent <id>` | The id of the task this task belongs to, empty to ungroup it |  |
| `--pull-requests <urls>` | Comma-separated urls of the pull requests opened for this task, replacing the list, empty to clear it |  |
| `--remove-pull-request <urls>` | Comma-separated urls of pull requests to remove from the ones opened for this task |  |
| `--remove-repo <paths>` | Comma-separated paths of repositories to remove from the ones this task changes |  |
| `--repos <paths>` | Comma-separated paths of the repositories this task changes, replacing the list, empty for all of them |  |
| `--status <status>` | New status (draft, todo, in-progress, fucked-up, unmerged, done) |  |
| `--ticket <ticket>` | The ticket the task came from, empty to clear it |  |
| `--title <title>` | New title |  |
| `--unblock <ids>` | Comma-separated ids of tasks to remove from the ones this task waits for |  |
