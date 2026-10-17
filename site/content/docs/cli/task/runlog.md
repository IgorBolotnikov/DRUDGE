---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task runlog
linkTitle: runlog
weight: 12
generated: true
---

```text
usage: drg task runlog <task-id> [options]
```

Print a compact view of the latest run of a task: its steps, one line each, and its stderr log when it has anything in it. A tool call that failed shows the first line of its error under it.

The task ID may be the short one a listing prints, as long as it names a single task.

## Arguments

- `<task-id>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `--raw` | Print the files the agent wrote as it wrote them, with JSON lines indented | `false` |
