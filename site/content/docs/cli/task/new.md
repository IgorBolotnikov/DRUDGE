---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task new
linkTitle: new
weight: 1
generated: true
---

```text
usage: drg task new [options]
```

Add a task to the project linked to the current directory.

## Options

| Option | Description | Default |
| --- | --- | --- |
| `--blocked-by <ids>` | Comma-separated ids of the tasks this task waits for |  |
| `--description <text>` | Description, the text the agent is handed as its prompt |  |
| `--description-file <path>` | Read the description from the file at path, - to read stdin |  |
| `--parent <id>` | The id of the task this task belongs to |  |
| `--pull-requests <urls>` | Comma-separated urls of the pull requests opened for this task |  |
| `--repos <paths>` | Comma-separated paths of the repositories this task changes, as drg project init prints them, all of them when left out |  |
| `--status <status>` | Task status (draft, todo, in-progress, fucked-up, unmerged, done), task.defaultStatus from the config when left out, todo when that is unset |  |
| `--ticket <ticket>` | The ticket the task came from |  |
| `--title <title>` | Task title |  |
