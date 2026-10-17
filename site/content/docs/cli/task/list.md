---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task list
linkTitle: list
weight: 2
generated: true
---

```text
usage: drg task list [options]
```

List tasks in the current project, one page at a time. Tasks belonging to another task are listed under it.

A filter lists the tasks it keeps without grouping them.

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-p, --page <number>` | The number of the page to show | `1` |
| `--page-size <items>` | How many items one page shows, 0 shows them all, task.pageSize from the project config or the global config when left out |  |
| `--parent <id>` | Filter by the id of the task the tasks belong to |  |
| `--status <status>` | Filter by status (draft, todo, in-progress, fucked-up, unmerged, done) |  |
| `--ticket <ticket>` | Filter by ticket ID |  |
